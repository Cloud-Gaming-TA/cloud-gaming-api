package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/AdityaP1502/Instant-Messanging/api/cache"
	httpx "github.com/AdityaP1502/Instant-Messanging/api/http"
	"github.com/AdityaP1502/Instant-Messanging/api/http/httputil"
	"github.com/AdityaP1502/Instant-Messanging/api/http/router"
	"github.com/AdityaP1502/Instant-Messanging/api/service/games/config"
	"github.com/AdityaP1502/Instant-Messanging/api/service/games/routes"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
)

var (
	PASSPHRASE       = "PASSPHRASE_PATH"
	CERT_FILE_PATH   = "CERT_FILE_PATH"
	PRIVATE_KEY_PATH = "PRIVATE_KEY_PATH"
	ROOT_CA_CERT     = "ROOT_CA_CERT"
)

func main() {
	path := "config/app.config.json"
	config, err := config.ReadJSONConfiguration(path)

	if err != nil {
		log.Fatal(err)
	}

	// cert, pKey := httputil.LoadCertificate(
	// 	os.Getenv(CERT_FILE_PATH),
	// 	os.Getenv(PRIVATE_KEY_PATH),
	// 	os.Getenv(PASSPHRASE),
	// )

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		config.Database.Host, config.Database.Port, config.Database.Username, config.Database.Password, config.Database.Database)

	db, err := sqlx.Open("postgres", psqlInfo)

	if err != nil {
		panic(err)
	}

	redis := cache.NewRedisClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Cache.Host, config.Cache.Port),
		Password: config.Cache.Password,
		DB:       0,
	})

	for {
		err = db.Ping()
		if err != nil {
			fmt.Println(err)
			fmt.Println("Failed to connect to the database. Retrying...")
			time.Sleep(time.Second)
			continue
		}

		err = redis.Client.Ping(redis.Context).Err()
		if err != nil {
			fmt.Println(err)
			fmt.Println("Failed to connect to redis. Retrying...")
			time.Sleep(time.Second)
			continue
		}
		break
	}

	// r := mux.NewRouter()

	r := router.CreateRouterx(&httpx.Metadata{
		DB:     db.DB,
		Cache:  redis,
		Config: config,
	})

	// load ca cert pool
	caCertPool := httputil.LoadRootCACertPool(os.Getenv(ROOT_CA_CERT))
	passphrasePath := os.Getenv(PASSPHRASE)

	if passphrasePath == "" {
		passphrasePath = "/tmp/passphrase"
	}

	cert, pKey := httputil.LoadCertificate(
		os.Getenv(CERT_FILE_PATH),
		os.Getenv(PRIVATE_KEY_PATH),
		passphrasePath,
	)

	// outbound tls config (to internal service)
	config.Config = &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{cert.Raw},
				PrivateKey:  pKey,
			},
		},

		RootCAs: caCertPool,
	}

	routes.SetGamesRoute(r.PathPrefix("/v1").Subrouter())
	// // r.Handle("/", r)

	// r.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
	// 	fmt.Println("WTF is happening")
	// 	w.Write([]byte("Hello, world!"))
	// 	w.WriteHeader(200)
	// }).Methods("GET")

	// a := s.PathPrefix("/account").Subrouter()

	// a.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	// 	w.Write([]byte("Hello, world!"))
	// }).Methods("GET")

	// wait until the server has ended

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		ExposedHeaders:   []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           86400, // time in seconds
	}).Handler(r.Router)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		if strings.ToLower(config.Server.Secure) == "true" {
			// inbound tls config (from proxy)
			tlsConfig := &tls.Config{
				Certificates: []tls.Certificate{
					{
						Certificate: [][]byte{cert.Raw},
						PrivateKey:  pKey,
					},
				},
				ClientCAs:  caCertPool,
				MinVersion: tls.VersionTLS10,
				ClientAuth: tls.RequestClientCert,
			}

			srv := http.Server{
				Addr:      fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port),
				Handler:   corsHandler,
				TLSConfig: tlsConfig,
			}

			srv.ListenAndServeTLS("", "")
		} else {
			err := http.ListenAndServe(
				fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port),
				corsHandler,
			)

			if err != nil {
				panic(err)
			}
		}
	}()

	fmt.Printf("Server is running on %s:%d\n", config.Server.Host, config.Server.Port)
	wg.Wait()
}
