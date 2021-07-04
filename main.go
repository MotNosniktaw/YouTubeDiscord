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
	"sort"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
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

func getChannelIdsForUserSubscriptions(service *youtube.Service, part []string) []string {
	call := service.Subscriptions.List(part)
	call = call.Mine(true)
	call = call.MaxResults(50)

	response, err := call.Do()
	handleError(err, "")

	channelIds := make([]string, 0)

	for _, item := range response.Items {
		channelIds = append(channelIds, item.Snippet.ResourceId.ChannelId)
	}

	return channelIds
}

func getUploadsForChannel(service *youtube.Service, part []string, channelId string) []*youtube.PlaylistItem {
	fmt.Println(fmt.Sprintf("Channel id: %s", channelId))
	getChannelCall := service.Channels.List(part)
	getChannelCall = getChannelCall.Id(channelId)

	getChannelResponse, err := getChannelCall.Do()
	handleError(err, "")

	fmt.Println(fmt.Sprintf("The id of the upload playlist: %s", getChannelResponse.Items[0].ContentDetails.RelatedPlaylists.Uploads))
	getUploadsCall := service.PlaylistItems.List(part)
	getUploadsCall = getUploadsCall.PlaylistId(getChannelResponse.Items[0].ContentDetails.RelatedPlaylists.Uploads)

	getUploadsResponse, err := getUploadsCall.Do()
	handleError(err, "")

	sort.Slice(getUploadsResponse.Items, func(a, b int) bool {
		return getUploadsResponse.Items[a].Snippet.PublishedAt > getUploadsResponse.Items[b].Snippet.PublishedAt
	})

	// for _, upload := range getUploadsResponse.Items {
	// 	// fmt.Println(fmt.Sprintf("The upload id: %s, tittle: %s, publish time: %s", upload.Id, upload.Snippet.Title, upload.Snippet.PublishedAt))
	// }

	return getUploadsResponse.Items
}

func main() {
	fmt.Println("Hi")
	err := godotenv.Load(".env")

	if err != nil {
		_, tokenFound := os.LookupEnv("YT_BOT_DISCORD_TOKEN")
		_, channelIdFound := os.LookupEnv("YT_BOT_DISCORD_CHANNELID")
		if !tokenFound || !channelIdFound {
			handleError(err, "")
		}
	}

	fmt.Println(os.Getenv("YT_BOT_DISCORD_TOKEN"))
	fmt.Println(os.Getenv("YT_BOT_DISCORD_CHANNELID"))
	db, err := discordgo.New("Bot " + os.Getenv("YT_BOT_DISCORD_TOKEN"))
	db.Open()

	db.ChannelMessageSend(os.Getenv("YT_BOT_DISCORD_CHANNELID"), "I'm heeeerree!")

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

	part := []string{"snippet", "contentDetails"}
	// channelsListByUsername(service, part, "GoogleDevelopers")

	lastPostedUploads := make(map[string]string)

	for true {
		channels := getChannelIdsForUserSubscriptions(service, []string{"snippet", "contentDetails"})
		for _, channelId := range channels {
			uploads := getUploadsForChannel(service, part, channelId)

			if lastPostedUploads[channelId] != "" {
				uploadsToPublish := make([]*youtube.PlaylistItem, 0)
				for _, upload := range uploads {
					if upload.Snippet.PublishedAt > lastPostedUploads[channelId] {
						uploadsToPublish = append(uploadsToPublish, upload)
					}
				}

				if len(uploadsToPublish) == 0 {
					fmt.Println("No new uploads")
				} else {
					for _, upload := range uploadsToPublish {
						fmt.Println(fmt.Sprintf("Do thing that publishes link. Id: %s", upload.Snippet.ResourceId.VideoId))
						db.ChannelMessageSend(os.Getenv("YT_BOT_DISCORD_CHANNELID"), "https://youtube.com/watch?v="+upload.Snippet.ResourceId.VideoId)
					}
				}
			} else {
				fmt.Println("No Previous posts. Will record from this point on.")
			}
			lastPostedUploads[channelId] = uploads[0].Snippet.PublishedAt

			time.Sleep(10 * time.Second)
		}
	}
}
