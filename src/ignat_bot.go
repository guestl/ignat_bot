package main

import (
    "github.com/go-telegram-bot-api/telegram-bot-api"
    "log"
    "os"
    "encoding/json"
	"database/sql"
	_ "github.com/lib/pq"
)

type Config struct {
    TelegramBotToken string
}


func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

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

	connectionString := "user=ignat_db password=ignatStrongPassword dbname=ignat_db"
	ignatDB, err := sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatal(err)
	}
	defer ignatDB.Close()

	isTrustedQuery, err := ignatDB.Prepare("select is_trusted from ignated_chat_users where chat_id = $1 and user_id = $2")
	if err != nil {
		log.Fatal(err)
	}
	defer isTrustedQuery.Close()
	var isTrustedUser bool

	addUntrustedQuery, err := ignatDB.Prepare("insert into ignated_chat_users (chat_id, user_id) values ($1, $2)")
	if err != nil {
		log.Fatal(err)
	}
	defer addUntrustedQuery.Close()


	updates, err := bot.GetUpdatesChan(updateFromBot)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		log.Printf("from [%s] was message.Text: %s", update.Message.From.UserName, update.Message.Text)
		log.Printf("from [%s] was message.Caption: %s", update.Message.From.UserName, update.Message.Caption)
//		log.Printf("from [%s] was message: %s", update.Message.From.UserName, update.Message.Entities)

//		if update.Message.IsCommand() {
//			switch update.Message.Command() {
//			case "help":
//				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
//				msg.Text = "type /sayhi, /status "
//				bot.Send(msg)
//			case "sayhi":
//				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
//				msg.Text = "Hi :)"
//				bot.Send(msg)
//			default:
//				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
//				msg.Text = "Unknown command"
//				bot.Send(msg)
//			}
//		}

		var isSpamer bool = false
		isTrustedUser = false

/*
		rows, err := isTrustedQuery.Query(update.Message.Chat.ID, newUserId.ID)
		if err != nil {
			log.Fatal(err)
		}
		if rows != nil {
			for rows.Next() {
				err := rows.Scan(&isTrustedUser)
				if err != nil {
					log.Fatal(err)
				}
			}
			err = rows.Err()
			if err != nil {
				log.Fatal(err)
			}
		}
		log.Printf("for %s isTrustedUser == %s", newUserId.ID, isTrustedUser)
*/
		if isTrustedUser == false {
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
		}


		if update.Message.NewChatMembers !=nil {
			for _, newUserId := range *update.Message.NewChatMembers {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
				msg.Text = "привет, кремлебот, " + newUserId.FirstName 
				msg.ReplyToMessageID = update.Message.MessageID
				bot.Send(msg)

				log.Println("check is user in db")
				// проверка, есть ли вообще
				isTrustedRows, err := isTrustedQuery.Query(update.Message.Chat.ID, newUserId.ID)
				if err != nil {
					log.Fatal(err)
				}
				log.Println("check finished")
				log.Printf("isTrustedRows is %s", isTrustedRows)
				
				// если проверка вернула ErrNoRows -> юзера нет в бд, запишем как антраста
				for isTrustedRows.Next() {
					err := isTrustedRows.Scan(&isTrustedUser)
					if err != nil {
						log.Fatal(err)
						if err == sql.ErrNoRows {
							log.Printf("add to chat %s new userid %s", update.Message.Chat.ID, newUserId.ID)
							_, err := addUntrustedQuery.Exec(update.Message.Chat.ID, newUserId.ID)
							if err != nil {
								log.Fatal(err)
							log.Println("theoritically added")
							}
						}
					}
					log.Printf("err in Scan is %s", err)
				}
			}
		}
	}
}
