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

type RPC struct {
	Name   string            `yaml:"name"` // Short rpc name (what do you want)
	Path   string            `yaml:"path"` // link/path to local/online ethereum network
	Client *ethclient.Client // Ethereum client
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

	var Config = Config_struct{Timeout: time.Second}
	// Parse yaml
	err = yaml.Unmarshal(data, &Config)
	if err != nil {
		log.Fatalln("Parsing yaml:\n\t", err)
	}
	Config.Timeout = time.Millisecond * Config.Timeout

	var (
		background = context.Background()
		ctx        context.Context
		cancel     context.CancelFunc
	)

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
		if err != nil {
			log.Fatalln("Testing client by calling last block:", rpc.Path, "\n\t", err)

		}

		Config.Dials[i] = rpc
	}

	// Telegram part
	// Connect to api
	bot, err := tgbotapi.NewBotAPI(Config.Token)
	if err != nil {
		log.Fatalln("Connecting to api:\n\t", err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	log.Println("Started successfully")

	// Recieve channel updates
	for update := range bot.GetUpdatesChan(u) {
		go CommandHandler(bot, update, &Config)
	}
}

var valid_adress = regexp.MustCompile("^0x[0-9a-fA-F]{40}$")

func CommandHandler(bot *tgbotapi.BotAPI, update tgbotapi.Update, config *Config_struct) {
	var err error

	if update.Message != nil && update.Message.IsCommand() {

		switch update.Message.Command() {
		// Balance
		case "balance":
			adress := update.Message.CommandArguments()
			// If adress is not valid
			if !valid_adress.MatchString(adress) {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Wallet adress is *not valid*!")
				msg.ReplyToMessageID = update.Message.MessageID
				msg.ParseMode = tgbotapi.ModeMarkdown
				_, err = bot.Send(msg)
				if err != nil {
					log.Println("/balance: Trying to send 'incorrect wallet' message\n\t", err)
				}
				return
			}

			// Get all dials
			var (
				background = context.Background()
				ctx        context.Context
				cancel     context.CancelFunc
				// Wallet adress
				hex_adress = common.HexToAddress(adress)
				// Balance
				balance = new(big.Int)
				// Telegram bot output
				output = time.Now().Format("🗓 *2006-01-02*\n⌚️ *3:04 PM* (GMT+1)\n\n")
			)

			for _, rpc := range config.Dials {
				// Update context
				ctx, cancel = context.WithTimeout(background, time.Duration(config.Timeout))
				// Get balance with timeout
				balance, err = rpc.Client.BalanceAt(ctx, hex_adress, nil)
				// Cancel context
				cancel()

				if err != nil {
					log.Printf("%s (%s):\n\t%s", rpc.Name, rpc.Path, err)

					switch err.(type) {
					// If server error
					case *url.Error:
						output += fmt.Sprintf("*%s*:  `%s`\n", rpc.Name, "_service error_")
					}
				}
				// If balance not equal 0
				if balance.String() != "0" {
					output += fmt.Sprintf("*%s*:  `%s`\n", rpc.Name, decimal.NewFromBigInt(balance, -18))
				}
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, output)
			msg.ReplyToMessageID = update.Message.MessageID
			msg.ParseMode = tgbotapi.ModeMarkdown
			_, err = bot.Send(msg)
			if err != nil {
				log.Println("/balance: Trying to send reply message\n\t", err)
			}

		// Help - all commands
		case "help":
			msg := tgbotapi.NewMessage(update.Message.Chat.ID,
				`*Commands:*
/balance: view adress eth amount
	We check all available rpc's
`,
			)
			msg.ReplyToMessageID = update.Message.MessageID
			msg.ParseMode = tgbotapi.ModeMarkdown
			_, err = bot.Send(msg)
			if err != nil {
				log.Println("/help: Trying to send reply message\n\t", err)
			}

		// If no one substituted:
		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Command *not found!*\nType /help to see any")
			msg.ReplyToMessageID = update.Message.MessageID
			msg.ParseMode = tgbotapi.ModeMarkdown
			_, err = bot.Send(msg)
			if err != nil {
				log.Println("Trying to send 'not found, see /help' message\n\t", err)
			}
		}
	}
}
