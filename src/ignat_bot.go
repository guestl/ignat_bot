package main

import (
    "github.com/go-telegram-bot-api/telegram-bot-api"
    "log"
    "os"
    "encoding/json"
)

type Config struct {
    TelegramBotToken string
}

func main() {
	f, log_err := os.OpenFile("ignat_logfile.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if log_err != nil {
    	log.Fatalf("error opening file: %v", log_err)
	}
	defer f.Close()

	log.SetOutput(f)

    file, _ := os.Open("config.json")
    decoder := json.NewDecoder(file)
    configuration := Config{}
    err := decoder.Decode(&configuration)
    if err != nil {
       log.Panic(err)
    }

    bot, err := tgbotapi.NewBotAPI(configuration.TelegramBotToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = false

	log.Printf("Authorized on account %s", bot.Self.UserName)

	updateFromBot := tgbotapi.NewUpdate(0)
	updateFromBot.Timeout = 60

	updates, err := bot.GetUpdatesChan(updateFromBot)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		log.Printf("from [%s] was message.Text: %s", update.Message.From.UserName, update.Message.Text)
		log.Printf("from [%s] was message.Caption: %s", update.Message.From.UserName, update.Message.Caption)
//		log.Printf("from [%s] was message: %s", update.Message.From.UserName, update.Message.Entities)

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "help":
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
				msg.Text = "type /sayhi, /status "
				bot.Send(msg)
			case "sayhi":
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
				msg.Text = "Hi :)"
				bot.Send(msg)
			default:
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
				msg.Text = "Unknown command"
				bot.Send(msg)
			}
		}

		var isSpamer bool = false

		if update.Message.Entities != nil {
			for _, messageEntity := range *update.Message.Entities {
					if (messageEntity.IsUrl() || messageEntity.IsTextLink()){
						isSpamer = true
					}
			}			
		}


		if update.Message.CaptionEntities != nil {
			for _, captionEntity := range *update.Message.CaptionEntities {
					if (captionEntity.IsUrl() || captionEntity.IsTextLink()){
						isSpamer = true
					}
			}			
		}

		if(isSpamer){
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
			msg.Text = "нельзя ссылку давать"
			msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(msg)
		}

		if update.Message.NewChatMembers !=nil {
			for _, newUserId := range *update.Message.NewChatMembers {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
						msg.Text = "привет, кремлебот, " + newUserId.FirstName
						msg.ReplyToMessageID = update.Message.MessageID
						bot.Send(msg)
			}

		}

	}
}