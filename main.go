package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Manga struct {
	ID int `json:"id_serie"`
}

type Mangas []Manga

var bot *tgbotapi.BotAPI

var baseApi = os.Getenv("BASE_API")

func main() {
	bot, _ = tgbotapi.NewBotAPI(os.Getenv("TOKEN_BOT"))

	bot.Debug = false

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		userMessage := update.Message.Text

		if isStartMessage(userMessage) {

			send(chatID, getMessageStart())
			continue
		}

		if isInvalidMessage(userMessage) {
			send(chatID, "Sorry, enter a valid command 😵. Example: /manga naruto 1")
			continue
		}

		command := getCommandByMessage(userMessage)
		manga := getMangaByMessage(userMessage)
		chapter := getChapterByMessage(userMessage)

		send(chatID, "Finding manga 🔎")

		if isInvalidCommand(command) {
			send(chatID, "Sorry, enter a valid command 😵. Example: /manga naruto 1")
			continue
		}

		if isInvalidChapter(chapter) {
			bot.Send(tgbotapi.NewMessage(chatID, "Sorry, invalid chapter 😵"))
			continue
		}

		mangaID, err := getMangaID(manga)

		if err != nil {
			send(chatID, "Opps... Something went wrong 😵")
			continue
		}

		images, err := getImagesByManga(mangaID, chapter)

		if err != nil {
			send(chatID, "Opps... Something went wrong 😵")
			continue
		}

		send(chatID, "Downloading chapters 📖")

		if len(images) == 0 {
			send(chatID, "Opps... Something went wrong 😵")
			continue
		}

		for _, imageURL := range images {
			imageBytes, _ := getImageByURL(imageURL)
			sendImage(chatID, imageBytes)
		}

	}
}

func send(chatID int64, message string) {
	bot.Send(tgbotapi.NewMessage(chatID, message))
}

func sendImage(chatID int64, imageBytes []byte) {
	fileBytes := tgbotapi.FileBytes{Bytes: imageBytes}

	bot.Send(tgbotapi.NewPhoto(chatID, fileBytes))
}

func isStartMessage(message string) bool {
	return message == "/start"
}

func isInvalidMessage(message string) bool {
	return len(strings.Split(message, " ")) < 3
}

func getCommandByMessage(message string) string {
	return strings.Split(message, " ")[0]
}
func getMangaByMessage(message string) string {
	parts := strings.Split(message, " ")
	words := parts[1 : len(parts)-1]
	return strings.Join(words, " ")
}
func getChapterByMessage(message string) string {
	parts := strings.Split(message, " ")
	return parts[len(parts)-1]
}

func isInvalidCommand(command string) bool {
	return command != "/manga"
}

func isInvalidChapter(chapter string) bool {
	return !regexp.MustCompile(`^[-.0-9]+$`).MatchString(chapter)
}

func getMangaID(manga string) (int, error) {
	var mangas Mangas
	res, err := http.Get(fmt.Sprintf("%s/mangas/%s", baseApi, manga))

	if err != nil {
		return 0, err
	}

	defer res.Body.Close()
	json.NewDecoder(res.Body).Decode(&mangas)

	return mangas[0].ID, nil
}

func getImagesByManga(mangaID int, chapter string) ([]string, error) {
	images := []string{}

	res, err := http.Get(fmt.Sprintf("%s/manga/%v/%s", baseApi, mangaID, chapter))

	if err != nil {
		return images, err
	}

	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&images)

	return images, err
}

func getImageByURL(imageURL string) ([]byte, error) {
	res, err := http.Get(imageURL)

	if err != nil {
		return []byte{}, err
	}

	defer res.Body.Close()

	bytes, err := ioutil.ReadAll(res.Body)

	return bytes, err
}

func getMessageStart() string {
	return fmt.Sprintf(`
Welcome to Animei Bot 😃

To begin type the command /manga.
Example: /manga naruto 1`,
	)
}
