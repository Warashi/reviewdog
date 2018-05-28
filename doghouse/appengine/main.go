package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/haya14busa/reviewdog/doghouse/server/cookieman"
	"github.com/haya14busa/reviewdog/doghouse/server/storage"
	"github.com/haya14busa/secretbox"
	"google.golang.org/appengine"
)

func mustCookieMan() *cookieman.CookieMan {
	// Create secret key by following command.
	// $ ruby -rsecurerandom -e 'puts SecureRandom.hex(32)'
	cipher, err := secretbox.NewFromHexKey(mustGetenv("SECRETBOX_SECRET"))
	if err != nil {
		log.Fatalf("failed to create secretbox: %v", err)
	}
	c := cookieman.CookieOption{
		http.Cookie{
			HttpOnly: true,
			Secure:   !appengine.IsDevAppServer(),
			MaxAge:   int((30 * 24 * time.Hour).Seconds()),
			Path:     "/",
		},
	}
	return cookieman.New(cipher, c)
}

func mustGitHubAppsPrivateKey() []byte {
	// Private keys https://github.com/settings/apps/reviewdog
	const privateKeyFile = "./secret/github-apps.private-key.pem"
	githubAppsPrivateKey, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		log.Fatalf("could not read private key: %s", err)
	}
	return githubAppsPrivateKey
}

func mustGetenv(name string) string {
	s := os.Getenv(name)
	if s == "" {
		log.Fatalf("%s is not set", name)
	}
	return s
}

func mustIntEnv(name string) int {
	s := os.Getenv(name)
	if s == "" {
		log.Fatalf("%s is not set", name)
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Fatal(err)
	}
	return i
}

func main() {
	integrationID := mustIntEnv("GITHUB_INTEGRATION_ID")
	ghPrivateKey := mustGitHubAppsPrivateKey()

	ghInstStore := storage.GitHubInstallationDatastore{}
	ghRepoTokenStore := storage.GitHubRepoTokenDatastore{}

	ghHandler := NewGitHubHandler(
		mustGetenv("GITHUB_CLIENT_ID"),
		mustGetenv("GITHUB_CLIENT_SECRET"),
		mustCookieMan(),
		ghPrivateKey,
		integrationID,
	)

	ghChecker := githubChecker{
		privateKey:       ghPrivateKey,
		integrationID:    integrationID,
		ghInstStore:      &ghInstStore,
		ghRepoTokenStore: &ghRepoTokenStore,
	}

	ghWebhookHandler := githubWebhookHandler{
		secret:      []byte(mustGetenv("GITHUB_WEBHOOK_SECRET")),
		ghInstStore: &ghInstStore,
	}

	http.HandleFunc("/", handleTop)
	http.HandleFunc("/check", ghChecker.handleCheck)
	http.HandleFunc("/gh_/webhook", ghWebhookHandler.handleWebhook)
	http.HandleFunc("/gh_/auth/callback", ghHandler.HandleAuthCallback)
	http.Handle("/gh/", ghHandler.Handler(http.HandlerFunc(ghHandler.HandleGitHubTop)))
	appengine.Main()
}

func handleTop(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "reviewdog")
}
