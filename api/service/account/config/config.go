package config

import (
	"crypto/tls"
	"encoding/base64"
	"os"
	"path/filepath"

	"github.com/AdityaP1502/Instant-Messanging/api/jsonutil"
)

var (
	DB_PASSWORD     string = os.Getenv("DB_PASSWORD")
	DB_USER         string = os.Getenv("DB_USER")
	DB_DATABASE     string = os.Getenv("DB_DATABASE_NAME")
	HASH_SECRET_KEY string = os.Getenv("HASH_SECRET_KEY")
)

type ServiceAPI struct {
	Host   string `json:"host"`
	Port   int    `json:"port,string"`
	Scheme string `json:"scheme"`
}

type Config struct {
	ServiceName string `json:"service_name"`
	Version     string `json:"version"`
	Database    struct {
		Driver   string `json:"driver"`
		Host     string `json:"host"`
		Port     int    `json:"port,string"`
		Username string `json:"username"`
		Password string `json:"password"`
		Database string `json:"database"`
	} `json:"database"`

	Server struct {
		Host   string `json:"host"`
		Port   int    `json:"port,string"`
		Secure string `json:"secure"`
	} `json:"server"`

	Certificate struct {
		CertFile string `json:"certFile"`
		KeyFile  string `json:"KeyFile"`
	} `json:"certificate"`

	Services struct {
		Mail ServiceAPI `json:"mail"`
		Auth ServiceAPI `json:"auth"`
	}

	Hash struct {
		SecretKeyBase64 string `json:"secretKey"`
		SecretKeyRaw    []byte `json:"-"`
	} `json:"prehash"`

	OTP struct {
		ResendDurationMinutes int `json:"resendDurationMinutes,string"`
		OTPDurationMinutes    int `json:"otpDurationMinutes,string"`
	} `json:"otp"`

	*tls.Config
}

func ReadJSONConfiguration(path string) (*Config, error) {
	var config Config

	// Assign config from environment variables
	config.Database.Username = DB_USER
	config.Database.Password = DB_PASSWORD
	config.Database.Database = DB_DATABASE
	config.Hash.SecretKeyBase64 = HASH_SECRET_KEY

	// Read the other configuration from app.config.json
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}

	exePath := filepath.Dir(exe)

	configFile, err := os.Open(filepath.Join(exePath, "", path))

	if err != nil {
		return nil, err
	}

	defer configFile.Close()

	err = jsonutil.DecodeJSON(configFile, &config)

	if err != nil {
		return nil, err
	}

	// Convert secretkey base64 to raw
	key, err := base64.StdEncoding.DecodeString(config.Hash.SecretKeyBase64)

	if err != nil {
		return nil, err
	}

	config.Hash.SecretKeyRaw = key

	return &config, nil
}
