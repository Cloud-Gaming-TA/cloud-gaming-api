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

	httpx "github.com/AdityaP1502/Instant-Messanging/api/http"
	"github.com/AdityaP1502/Instant-Messanging/api/http/httputil"
	"github.com/AdityaP1502/Instant-Messanging/api/http/router"
	"github.com/AdityaP1502/Instant-Messanging/api/service/auth/config"
	"github.com/AdityaP1502/Instant-Messanging/api/service/auth/routes"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
)

var (
	PASSPHRASE       = "PASSPHRASE"
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

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		config.Database.Host, config.Database.Port, config.Database.Username, config.Database.Password, config.Database.Database)

	db, err := sqlx.Open("postgres", psqlInfo)

	if err != nil {
		panic(err)
	}

	for {
		err = db.Ping()
		if err != nil {
			fmt.Println(err)
			fmt.Println("Failed to connect to the database. Retrying...")
			time.Sleep(time.Second)
			continue
		}
		break
	}

	// r := mux.NewRouter()
	r := router.CreateRouterx(
		&httpx.Metadata{
			DB:     db.DB,
			Config: config,
		},
	)

	passphrasePath := os.Getenv(PASSPHRASE)

	if passphrasePath == "" {
		passphrasePath = "/tmp/passphrase"
	}

	rootCAPool := httputil.LoadRootCACertPool(os.Getenv(ROOT_CA_CERT))

	cert, pKey := httputil.LoadCertificate(
		os.Getenv(CERT_FILE_PATH),
		os.Getenv(PRIVATE_KEY_PATH),
		passphrasePath,
	)

	// set outbound tls configuration
	config.Config = &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{cert.Raw},
			PrivateKey:  pKey,
		}},
		RootCAs: rootCAPool,
	}

	routes.SetAuthRoute(r.PathPrefix("/v1").Subrouter())

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		ExposedHeaders:   []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           86400, // time in seconds
	}).Handler(r.Router)

	// wait until the server has ended
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		if strings.ToLower(config.Server.Secure) == "true" {

			// Inbound tls configuration
			tlsConfig := &tls.Config{
				Certificates: []tls.Certificate{
					{
						Certificate: [][]byte{cert.Raw},
						PrivateKey:  pKey,
					},
				},
				ClientCAs:  rootCAPool,
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
