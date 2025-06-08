package main

import (
	"fmt"
	"os"
	"encoding/json"
	"path/filepath"
)

type Config struct {
	MusicDir string `json:"music_dir"`
	Loop bool `json:"loop"`
	RpcPort string `json:"rpc_port"`
}

func get_config() *Config {
	dirpath := get_config_dirpath()
	filepath := filepath.Join(dirpath, "config.json")
	file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		panic(err)
	}

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	file.Close()

	if err != nil {
		set_default_config(&config)
		save_config(&config)
	}
	return &config
}

func set_default_config(config *Config) {
	fmt.Printf("Config not found setting default config\n")
	*config = Config{
		MusicDir: filepath.Join(os.Getenv("HOME"), "Music"),
		Loop: true,
		RpcPort: ":42069",
	}
}

func save_config(config *Config) error {
	dirpath := get_config_dirpath()
	filepath := filepath.Join(dirpath, "config.json")
	file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(&config)
	if err != nil {
		return err
	}
	return nil
}

func get_dir(path string) bool {
	file_info, err := os.Stat(path)
	if err != nil {
		fmt.Printf("Apollo: Creating config dir in %s\n", path)
		os.Mkdir(path, 0755)
		file_info, err = os.Stat(path)
		if err != nil {
			panic(err)
		}
	}
	return file_info.IsDir()
}

func get_config_dirpath() string {
	user_config, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	config_dirpath := filepath.Join(user_config, "apollo")
	get_dir(config_dirpath)
	return config_dirpath
}
