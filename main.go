package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chrissgon/goanime"
	"github.com/chrissgon/gomanga"
	"github.com/joho/godotenv"
	"github.com/machinebox/progress"

	goanimepkg "github.com/chrissgon/goanime/pkg"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type CommandFunc func(chatID int64, message *tgbotapi.Message)

var bot *tgbotapi.BotAPI

var templates = map[string][]string{
	"start": {
		"👋 Olá visitante",
		"",
		"Encontre animes e mangas com o Animei Bot 🤖",
		"",
		"📄 Comandos:",
		"",
		"Pesquise um manga.",
		"👉 /manga nome capitulo - Exemplo: manga Naruto 1",
		"",
		"Pesquise um anime",
		"👉 /anime nome episodio - Exemplo: anime Naruto 1",
		"",
		"Pesquise um anime dublado",
		"👉 /animedub nome episodio - Exemplo: animedub Naruto 1",
	},

	"finding":    {"🔎 Iniciando procura..."},
	"stop":       {"😉 Certo, a pesquisa foi cancelada"},
	"found":      {"😉 Encontrei"},
	"download":   {"⬇️ Iniciando o download"},
	"downloaded": {"🤩 Download concluído"},
	"finally":    {"😅 Quase lá..."},
	"progress":   {"✅ %d%% concluído..."},
	"nostop":     {"❌ Por favor, espere o download atual finalizar."},
	"inprocess":  {"❌ Há uma pesquisa em andamento. Envie o comando /stop antes de iniciar outra."},
	"notfound":   {"😕 Desculpe, não pude encontrar nada..."},
	"error":      {"😵 Opps... Alguma coisa deu errado"},
	"nocommand":  {"😵 Desculpe, insira um comando válido, como /start"},
	"noparams":   {"😵 Desculpe, parametros inválidos"},
}

var commands = map[string]CommandFunc{
	"start":    execStartCommand,
	"stop":     execStopCommand,
	"manga":    execMangaCommand,
	"anime":    execAnimeCommand,
	"animedub": execAnimeDubCommand,
}

var process = map[int64]string{}

func init() {
	if godotenv.Load() != nil {
		panic("error load .env")
	}
}

func main() {
	wg := sync.WaitGroup{}

	wg.Add(1)

	go botServer()
	go playServer()

	wg.Wait()
}

func playServer() {
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	err := http.ListenAndServe(":3000", nil)
	fmt.Println(err)
}

func botServer() {
	bot, _ = tgbotapi.NewBotAPI(os.Getenv("TOKEN_BOT"))

	bot.Debug = false

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		processChannelUpdates(update)
	}
}

func processChannelUpdates(update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	message := update.Message
	command := message.Command()
	// receiveCommandWithParams := command == "manga" || command == "anime" || command == "animedub"
	// receiveStopCommand := command == "stop"

	if !message.IsCommand() {
		send(chatID, getTemplate("nocommand"))
		return
	}

	if command == "stop" {
		setCurrentCommandProcess(chatID, command)
		commands[command](chatID, message)
		return
	}

	if existsProcess(chatID) {
		fmt.Println("existprocess")
		send(chatID, getTemplate("inprocess"))
		return
	}

	// if receiveStopCommand && currentCommandProcessIs(chatID, "anime") || currentCommandProcessIs(chatID, "animedub") {
	// 	send(chatID, getTemplate("nostop"))
	// 	return
	// }

	// if !receiveStopCommand && existsProcess(chatID) {
	// }

	// if receiveCommandWithParams && paramsIsInvalid(getParams(message.CommandArguments())) {
	// 	send(chatID, getTemplate("noparams"))
	// 	return
	// }

	setCurrentCommandProcess(chatID, command)

	go func() {
		commands[command](chatID, message)
		finishCurrentCommandProcess(chatID)
	}()
	// finishCurrentCommandProcess(chatID)
}

/*
##################
##################

	COMMANDS

##################
##################
*/
func execStartCommand(chatID int64, message *tgbotapi.Message) {
	send(chatID, getTemplate("start"))
}
func execStopCommand(chatID int64, message *tgbotapi.Message) {
	time.Sleep(2 * time.Second)
	send(chatID, getTemplate("stop"))
}

func execMangaCommand(chatID int64, message *tgbotapi.Message) {
	manga, chapter := getParams(message.CommandArguments())

	if paramsIsInvalid(manga, chapter) {
		send(chatID, getTemplate("noparams"))
		return
	}

	send(chatID, getTemplate("finding"))

	images, err := gomanga.SearchAll(manga, chapter)

	if err != nil {
		send(chatID, getTemplate("notfound"))
		return
	}

	send(chatID, getTemplate("found"))

	for _, imageURL := range images {
		if stopCurrentProccess(chatID) {
			break
		}

		_, err := sendImage(chatID, imageURL)

		if err != nil {
			send(chatID, getTemplate("error"))
			break
		}
	}
}

func execAnimeCommand(chatID int64, message *tgbotapi.Message) {
	anime, episode := getParams(message.CommandArguments())

	if paramsIsInvalid(anime, episode) {
		send(chatID, getTemplate("noparams"))
		return
	}

	send(chatID, getTemplate("finding"))

	scrapers := goanime.NewScrapers(anime, episode, false)

	initDownloadAnimeProcess(chatID, scrapers)
}

func execAnimeDubCommand(chatID int64, message *tgbotapi.Message) {
	anime, episode := getParams(message.CommandArguments())

	if paramsIsInvalid(anime, episode) {
		send(chatID, getTemplate("noparams"))
		return
	}

	send(chatID, getTemplate("finding"))

	scrapers := goanime.NewScrapers(anime, episode, true)

	initDownloadAnimeProcess(chatID, scrapers)
}

/*
##################
##################
	OTHERS
##################
##################
*/

func setCurrentCommandProcess(chatID int64, command string) {
	fmt.Println("setCurrentCommandProcess", command)
	process[chatID] = command
}
func finishCurrentCommandProcess(chatID int64) {
	fmt.Println("finishCurrentCommandProcess", chatID)
	delete(process, chatID)
}
func currentCommandProcessIs(chatID int64, command string) bool {
	return process[chatID] == command
}
func stopCurrentProccess(chatID int64) bool {
	return process[chatID] == "stop"
	// currentProcessIsStop := process[chatID] == "stop"
	// if currentProcessIsStop {
	// 	finishCurrentCommandProcess(chatID)
	// 	return currentProcessIsStop
	// }
	// return false
}
func existsProcess(chatID int64) bool {
	_, exists := process[chatID]
	return exists
}

func initDownloadAnimeProcess(chatID int64, scrapers []goanimepkg.Scraper) {
	status := make(chan progress.Progress)

	go alertProgressDownload(chatID, status)

	filepath, err := goanime.DownloadByScrapers(scrapers, status)

	if err != nil {
		send(chatID, getTemplate("notfound"))
		return
	}

	urlVideo := fmt.Sprintf("https://clever-shoes-cheat.loca.lt/%s", filepath[9:])

	buttonLink := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("Assistir Vídeo", urlVideo),
		),
	)
	message := tgbotapi.NewMessage(chatID, getTemplate("downloaded"))
	message.ReplyMarkup = buttonLink
	bot.Send(message)
}

func alertProgressDownload(chatID int64, status chan progress.Progress) {
	calls := 1
	first := true
	for s := range status {
		percent := int(s.Percent())

		if first {
			first = false
			send(chatID, getTemplate("download"))
			continue
		}

		if percent > calls*25 {
			calls++
			send(chatID, parseTemplate(getTemplate("progress"), percent))
		}
	}
}

func send(chatID int64, message string) (tgbotapi.Message, error) {
	return bot.Send(tgbotapi.NewMessage(chatID, message))
}

func sendImage(chatID int64, url string) (tgbotapi.Message, error) {
	res, err := http.Get(url)

	if err != nil {
		return tgbotapi.Message{}, err
	}

	file := tgbotapi.FileReader{
		Name:   "",
		Reader: res.Body,
	}

	return bot.Send(tgbotapi.NewPhoto(chatID, file))
}

func sendVideoURL(chatID int64, url string) (tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(chatID, "")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardButtonURL("Assistir video", url)
	return bot.Send(msg)
}

func getTemplate(template string) string {
	return strings.Join(templates[template], "\n")
}

func parseTemplate(template string, params ...any) string {
	return fmt.Sprintf(template, params...)
}

func getParams(text string) (string, string) {
	parts := strings.Split(text, " ")
	title := strings.Join(parts[:len(parts)-1], " ")
	number := parts[len(parts)-1]
	return title, number
}

func paramsIsInvalid(title string, number string) bool {
	return title == "" || number == ""
}

func getImageByURL(imageURL string) ([]byte, error) {
	res, err := http.Get(imageURL)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	bytes, err := ioutil.ReadAll(res.Body)

	return bytes, err
}
