package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"

	"google.golang.org/api/youtube/v3"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const missingClientSecretMessage = `Please configure OAuth2.0`

func getClient(ctx context.Context, config *oauth2.Config) *http.Client {
	cacheFile, err := tokenCacheFile()

	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}

	tok, err := getTokenFromFile(cacheFile)

	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(cacheFile, tok)
	}
	return config.Client(ctx, tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+"authorization code: \n%v\n", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read authorization code %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web %v", err)
	}

	return tok
}

func getTokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	t := &oauth2.Token{}

	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

func tokenCacheFile() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	tokenCacheDir := filepath.Join(usr.HomeDir, ".credentials")
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir, url.QueryEscape("youtube-go.quickstart.json")), err
}

func saveToken(file string, token *oauth2.Token) {
	fmt.Println("Saving credential to: %s\n", file)
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func handleError(err error, message string) {
	if message == "" {
		message = "Error making API call"
	}
	if err != nil {
		log.Fatalf(message+": %v", err.Error())
	}
}

func channelsListByUsername(service *youtube.Service, part []string, forUsername string) {
	call := service.Channels.List(part)
	call = call.ForUsername(forUsername)

	response, err := call.Do()
	handleError(err, "")
	fmt.Println(fmt.Sprintf("This channel's ID is %s. Its title is '%s', "+"and it has %d views.",
		response.Items[0].Id,
		response.Items[0].Snippet.Title,
		response.Items[0].Statistics.ViewCount))
}

func main() {
	fmt.Println("Hi")

	ctx := context.Background()

	b, err := ioutil.ReadFile("client_secret.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, youtube.YoutubeReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: $v", err)
	}

	client := getClient(ctx, config)
	service, err := youtube.New(client)

	handleError(err, "Error creating YouTube client")

	part := []string{"snippet", "contentDetails", "statistics"}
	channelsListByUsername(service, part, "GoogleDevelopers")
}
