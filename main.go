package main

import (
	"fmt"

	"github.com/adamararcane/Gator/internal/config"
)

func main() {
	cfgFile, err := config.Read()
	if err != nil {
		err = fmt.Errorf("error reading config: %w", err)
		fmt.Println(err)
	}

	cfgFile.SetUser("Adamar")

	cfgFile, err = config.Read()
	if err != nil {
		err = fmt.Errorf("error reading config: %w", err)
		fmt.Println(err)
	}
	fmt.Println(cfgFile)

}

type state struct {
	cfg *config.Config
}

type command struct {
	name string
	args []string
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		fmt.Println("Usage: gator login <username>")
		return fmt.Errorf("login expects 1 argument")
	}

}
