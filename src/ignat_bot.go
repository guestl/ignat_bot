package main

import (
    "github.com/go-telegram-bot-api/telegram-bot-api"
    "log"
    "os"
    "encoding/json"
	"database/sql"
	 _ "github.com/mattn/go-sqlite3"
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

	connectionString := "./db/ignat_db.db"
	ignatDB, err := sql.Open("sqlite3", connectionString)
	if err != nil {
		log.Fatal(err)
	}
	defer ignatDB.Close()

	isTrustedQuery := "select is_trusted from ignated_chat_users where chat_id = $1 and user_id = $2"
	var isTrustedUser bool

	addUntrustedQuery := "insert into ignated_chat_users (chat_id, user_id) values ($1, $2)"

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

		var isLinkInMessage bool = false
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
							isLinkInMessage = true
						}
				}			
			}


			if update.Message.CaptionEntities != nil {
				for _, captionEntity := range *update.Message.CaptionEntities {
						if (captionEntity.IsUrl() || captionEntity.IsTextLink()){
							isLinkInMessage = true
						}
				}			
			}

			if(isLinkInMessage){
				log.Println("check is user in db")
				// проверка, есть ли вообще
				log.Println(update.Message.From.ID)
				isTrustedRows := ignatDB.QueryRow(isTrustedQuery, update.Message.Chat.ID, update.Message.From.ID)
				err = isTrustedRows.Scan(&isTrustedUser)
				switch err{
				case sql.ErrNoRows:
					isTrustedUser = false
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
					msg.Text = "нельзя ссылку давать"
					msg.ReplyToMessageID = update.Message.MessageID
					bot.Send(msg)
					log.Println("sql.ErrNoRows case")
				default:
					log.Println("default case")
					log.Fatal(err)
				}
				log.Println("check finished")
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
				isTrustedRows := ignatDB.QueryRow(isTrustedQuery, update.Message.Chat.ID, newUserId.ID)
				err = isTrustedRows.Scan(&isTrustedUser)
				switch err{
				case sql.ErrNoRows:
					isTrustedUser = false
					log.Printf("add to chat %s new userid %s", update.Message.Chat.ID, newUserId.ID)
					result, err := ignatDB.Exec(addUntrustedQuery, update.Message.Chat.ID, newUserId.ID)
					if err != nil {
						log.Fatal(err)
					}
					log.Println("theoretically added")
					log.Println(result.RowsAffected())  // количество обновленных строк
				default:
					log.Fatal(err)
				}
				log.Println("check finished")
			}
		}
	}
}
