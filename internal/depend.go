package internal

import (
	"log"
	"net/http"

	"auth/config"

	"github.com/coreos/go-oidc/v3/oidc"
)

type Application struct {
	InfoLog    *log.Logger
	ErrorLog   *log.Logger
	Verifier   *oidc.IDTokenVerifier
	Config     *config.Config
	HTTPClient *http.Client
}
