/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"encoding/hex"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"io/ioutil"
	"net/http"
	"swagg/common"
	"swagg/encryption"
	"time"

	"github.com/spf13/viper"
	"github.com/hashicorp/vault/api"
)

var cfgFile string
var logger *common.Logger

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "swagg",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.swagg.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {

		// Search config in home directory with name ".swagg" (without extension).
		viper.AddConfigPath(".swagghome")
		viper.SetConfigType("toml")
		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("SWGCFG")
	viper.AutomaticEnv() // read in environment variables that match


	err := viper.ReadInConfig()

	logger = common.CreateNewLogger(false)

	if err != nil {

		logger.LogAndPanic(err)
	}


	GenerateSystemKeys()
	GetFromVault("systemkeys")




	//spew.Dump(viper.GetString("systemkeys.data.systemkeys"))


	GetGenesis()

	k := viper.AllKeys()

	spew.Dump(k)
	//fmt.Printf("generated system public key: %v", viper.Get("pubkey"))
	// If a config file is found, read it in.
	//if err := viper.ReadInConfig(); err == nil {

	logger.Get.WithFields(log.Fields{
		"configfile": viper.ConfigFileUsed(),
	}).Info("using config file:")

	logger.Get.WithFields(log.Fields{
		"genesis file": viper.GetString("genesisfile"),

	}).Info("using genesis file:")

	//}
}


func GenerateSystemKeys() {
	keypair := encryption.CreateNewKeyPair(encryption.Kyber)
	viper.Set("pubkey", hex.EncodeToString(keypair.PubKey))
	WriteToVault("systemkeys", hex.EncodeToString(keypair.PrivKey))


}

func GetGenesis() {

	genesisFile := viper.GetString("genesisfile")
	g, err := ioutil.ReadFile(".swagghome/"+genesisFile)

	if err != nil {

		logger.LogAndQuit(err)

	}

	viper.Set("genesis_block_data", g)

}

func GetVaultClient() *api.Client {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	token := viper.GetString("vault_token")
	vaultAddr := viper.GetString("vault_addr")
	client, err := api.NewClient(&api.Config{Address: vaultAddr, HttpClient: httpClient})
	if err != nil {
		logger.LogAndPanic(err)
	}
	client.SetToken(token)
	return client
}

func WriteToVault(key string, data string) {

	inputData := map[string]interface{}{

			"data": map[string]interface{}{key:data},

	}
	client := GetVaultClient()

	_, err := client.Logical().Write("secret/data/"+key, inputData)
	if err != nil {
		logger.LogAndPanic(err)
	}


}

func GetFromVault(key string) map[string]interface{} {

	client := GetVaultClient()
	data, err := client.Logical().Read("secret/data/"+key)
	if err != nil {
		logger.LogSoftError(err)

	}


			viper.Set("systemkeys", data.Data)



	return data.Data
}