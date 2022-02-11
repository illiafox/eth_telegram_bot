package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"regexp"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v2"
)

var (
	valid_adress = regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
	div_wei      = decimal.NewFromBigInt(big.NewInt(10), 18)
)

type RPC struct {
	name string `yaml:"name"`
	path string `yaml:"path"`
}
type Config_struct struct {
	token string `yaml:"name"`  // Bot token
	dials []RPC  `yaml:"dials"` // All ethereum connections
}

func main() {
	// Set and parse config flag (will be added more in future)
	config_path := flag.String("config", "config.yml", "open other config (instead of config.yml")
	flag.Parse()
	// read config file
	data, err := ioutil.ReadFile(*config_path)
	if err != nil {
		log.Fatalln("Opening config:\n", err)
	}
	// Main struct
	var Config Config_struct
	// Parse yaml
	err = yaml.Unmarshal(data, &Config)
	if err != nil {
		log.Fatalln("Parsing yaml:\n", err)
	}
	client, err := ethclient.Dial("https://www.ethercluster.com/etc")
	if err != nil {
		panic(err)
	}
	account := common.HexToAddress("0x78D5E220B4cc84f290Fae4148831b371a851a114")
	balance, err := client.BalanceAt(context.Background(), account, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(balance)
	d := decimal.NewFromBigInt(balance, 1)
	fmt.Println(d.Div(div_wei))

}
