package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/url"
	"regexp"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v2"
)

var (
	valid_adress = regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
	div_wei      = decimal.NewFromBigInt(big.NewInt(10), 18)
)

type RPC struct {
	Name   string `yaml:"name"`
	Path   string `yaml:"path"`
	Client *ethclient.Client
}
type Config_struct struct {
	Token   string        `yaml:"token"`   // Bot token
	Dials   []RPC         `yaml:"dials"`   // All ethereum connections
	Timeout time.Duration `yaml:"timeout"` // Context timeout in millisecond
}

func main() {
	// Set and parse config flag (will be added more in future)
	config_path := flag.String("config", "config.yml", "open other config (instead of config.yml")
	flag.Parse()
	// read config file
	data, err := ioutil.ReadFile(*config_path)
	if err != nil {
		log.Fatalln("Opening config:\n\t", err)
	}
	// Main struct
	var Config = Config_struct{Timeout: time.Second}
	// Parse yaml
	err = yaml.Unmarshal(data, &Config)
	if err != nil {
		log.Fatalln("Parsing yaml:\n\t", err)
	}
	// Update timeout
	Config.Timeout = time.Millisecond * Config.Timeout
	// Create context
	var (
		background = context.Background()
		ctx        context.Context
		cancel     context.CancelFunc
	)
	// Check all rpc's
	for i, rpc := range Config.Dials {
		ctx, cancel = context.WithTimeout(background, Config.Timeout)
		// Dial rpc client
		rpc.Client, err = ethclient.Dial(rpc.Path)
		if err != nil {
			log.Fatalln("Dialing with:", rpc.Path, "\n\t", err)

		}
		// Ping client by calling last block number (TODO: Find better alternative)
		_, err = rpc.Client.BlockNumber(ctx)
		// Close context
		cancel()
		// Check error
		if err != nil {
			log.Fatalln("Testing client by calling last block:", rpc.Path, "\n\t", err)

		}
		// If ok, set to config
		Config.Dials[i] = rpc
	}

	// Telegram part
	// Connect to api
	bot, err := tgbotapi.NewBotAPI(Config.Token)
	if err != nil {
		log.Fatalln("Connecting to api:\n\t", err)
	}
	// Set updates
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	// Log
	log.Println("Started successfully")
	// Recieve channel updates
	for update := range bot.GetUpdatesChan(u) {
		// call Handler with goroutine
		go CommandHandler(bot, update, &Config)
	}
}

func CommandHandler(bot *tgbotapi.BotAPI, update tgbotapi.Update, config *Config_struct) {
	var err error
	// If recieved message commmand
	if update.Message != nil && update.Message.IsCommand() {
		// Switch for command
		switch update.Message.Command() {

		// Balance
		case "balance":
			adress := update.Message.CommandArguments()
			// If adress is not valid
			if !valid_adress.MatchString(adress) {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Wallet adress is *not valid*!")
				// Set reply id
				msg.ReplyToMessageID = update.Message.MessageID
				// Set parse mode
				msg.ParseMode = tgbotapi.ModeMarkdown
				// Send message
				_, err = bot.Send(msg)
				if err != nil {
					log.Println("/balance: Trying to send 'incorrect wallet' message\n\t", err)
				}
				return
			}

			// Get all dials
			var (
				// Create context
				background = context.Background()
				ctx        context.Context
				cancel     context.CancelFunc
				// Wallet adress
				hex_adress = common.HexToAddress(adress)
				// Balance
				balance = new(big.Int)
				// Telegram bot output
				output = time.Now().Format("🗓 *2006-01-02*\n⌚️ *3:04* PM\n\n")
			)
			// Range rpc's
			for _, rpc := range config.Dials {
				// Update context
				ctx, cancel = context.WithTimeout(background, time.Duration(config.Timeout))
				// Get balance with timeout
				balance, err = rpc.Client.BalanceAt(ctx, hex_adress, nil)
				// Cancel context
				cancel()
				// Check error
				if err != nil {
					// Print Error
					log.Printf("%s (%s):\n\t%s", rpc.Name, rpc.Path, err)
					// Switch error
					switch err.(type) {
					// If server error
					case *url.Error:
						output += fmt.Sprintf("*%s*:  `%s`\n", rpc.Name, "_service error_")
					}
				}

				// If balance not equal 0
				if balance.String() != "0" {
					// add to output
					output += fmt.Sprintf("*%s*:  `%s`\n", rpc.Name, decimal.NewFromBigInt(balance, 1).Div(div_wei))
				}
			}
			// New Message
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, output)
			// Set reply id
			msg.ReplyToMessageID = update.Message.MessageID
			// Set parse mode
			msg.ParseMode = tgbotapi.ModeMarkdown
			// Send message
			_, err = bot.Send(msg)
			if err != nil {
				log.Println("/balance: Trying to send reply message\n\t", err)
			}

		// Help - all commands
		case "help":
			// Create message
			msg := tgbotapi.NewMessage(update.Message.Chat.ID,
				`*Commands:*
/balance: view adress eth amount
	We check all available rpc's
`,
			)
			// Set reply id
			msg.ReplyToMessageID = update.Message.MessageID
			// Set parse mode
			msg.ParseMode = tgbotapi.ModeMarkdown
			// Send message
			_, err = bot.Send(msg)
			if err != nil {
				log.Println("/help: Trying to send reply message\n\t", err)
			}

		// If no one substituted:
		default:
			// Create message
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Command *not found!*\nType `/help` to see any")
			// Set reply id
			msg.ReplyToMessageID = update.Message.MessageID
			// Set parse mode
			msg.ParseMode = tgbotapi.ModeMarkdown
			// Send message
			_, err = bot.Send(msg)
			if err != nil {
				log.Println("Trying to send 'not found, see /help' message\n\t", err)
			}
		}
	}
}
