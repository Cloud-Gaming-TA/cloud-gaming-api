package config

import (
	"crypto/tls"
	"encoding/base64"
	"os"
	"path/filepath"

	"github.com/AdityaP1502/Instant-Messanging/api/jsonutil"
)

var (
	DB_PASSWORD    string = os.Getenv("DB_PASSWORD")
	DB_USER        string = os.Getenv("DB_USER")
	DB_DATABASE    string = os.Getenv("DB_DATABASE_NAME")
	JWT_SECRET_KEY string = os.Getenv("JWT_SECRET_KEY")
)

type Service struct {
	Host string `json:"host"`
	Port int    `json:"port,string"`
}

type Config struct {
	ServiceName string `json:"service_name"`
	Version     string `json:"version"`
	Database    struct {
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

	Services struct {
		OCSP Service `json:"ocsp"`
	} `json:"services"`

	Certificate struct {
		CertFile string `json:"certFile"`
		KeyFile  string `json:"KeyFile"`
	} `json:"certificate"`

	Session struct {
		ExpireTime        int    `json:"expireTimeMinutes,string"`
		RefreshExpireTime int    `json:"refreshExpireTimeMinutes,string"`
		SecretKeyBase64   string `json:"secretKey"`
		SecretKeyRaw      []byte `json:"-"`
	} `json:"token"`

	*tls.Config
}

func ReadJSONConfiguration(path string) (*Config, error) {
	var config Config

	// Read config from environmet variables
	config.Database.Username = DB_USER
	config.Database.Password = DB_PASSWORD
	config.Database.Database = DB_DATABASE
	config.Session.SecretKeyBase64 = JWT_SECRET_KEY

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
	key, err := base64.StdEncoding.DecodeString(config.Session.SecretKeyBase64)

	if err != nil {
		return nil, err
	}

	config.Session.SecretKeyRaw = key

	return &config, nil
}
