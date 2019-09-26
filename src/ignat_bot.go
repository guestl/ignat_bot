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

	//isTrustedQuery := "select is_trusted from ignated_chat_users where chat_id = $1 and user_id = $2"
	var isTrustedUser bool
	var isLinkInMessage bool = false

	addUntrustedQuery := "insert into ignated_chat_users (chat_id, user_id) values ($1, $2)"
	updateTrustedQuery := "update ignated_chat_users set is_trusted = true where chat_id = $1 and user_id = $2"	
	getUsersQuery := "select chat_id, user_id, is_trusted from ignated_chat_users"

	updates, err := bot.GetUpdatesChan(updateFromBot)

	// загружаем списков юзеров всех чатов из бд в map of map
	var chatId int64
	var userId int
	mapOfAllUsersInDatabase := make(map[int64]map[int]bool)
	allUsersRows, err := ignatDB.Query(getUsersQuery)
	if err != nil {
		log.Fatal(err)
	}
	defer allUsersRows.Close()
	for allUsersRows.Next(){
		err = allUsersRows.Scan(&chatId, &userId, &isTrustedUser)
		if err != nil {
			log.Println(err)
			continue
		}
		log.Println(chatId, userId,  isTrustedUser)
		if mapOfAllUsersInDatabase[chatId] == nil{
			mapOfAllUsersInDatabase[chatId] = make(map[int]bool)
		}
		mapOfAllUsersInDatabase[chatId][userId] = isTrustedUser
	}

	log.Println(mapOfAllUsersInDatabase)

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

		isLinkInMessage = false
		isTrustedUser = mapOfAllUsersInDatabase[update.Message.Chat.ID][update.Message.From.ID]

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
				log.Println("we have a link from untrusted user")
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
				msg.Text = "нельзя ссылку давать"
				msg.ReplyToMessageID = update.Message.MessageID
				bot.Send(msg)
			} else{
				isTrustedUser = true
				log.Println("No link from untrusted user. Let's update the user as trusted")
				log.Printf("update in chat %s userid %s", update.Message.Chat.ID, update.Message.From.ID)
				result, err := ignatDB.Exec(updateTrustedQuery, update.Message.Chat.ID, update.Message.From.ID)
				if err != nil {
					log.Fatal(err)
				}

				if mapOfAllUsersInDatabase[update.Message.Chat.ID] == nil{
					mapOfAllUsersInDatabase[update.Message.Chat.ID] = make(map[int]bool)
				}
				mapOfAllUsersInDatabase[update.Message.Chat.ID][update.Message.From.ID] = isTrustedUser

				log.Println("theoretically user updated")
				log.Println(result.RowsAffected())  // количество обновленных строк
				log.Println(mapOfAllUsersInDatabase)

			}

		}

		if update.Message.NewChatMembers !=nil {
			for _, newUserId := range *update.Message.NewChatMembers {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
				msg.Text = "привет, кремлебот, " + newUserId.FirstName 
				msg.ReplyToMessageID = update.Message.MessageID
				bot.Send(msg)

				log.Println("we have a new user. check its id user in the db")
				// проверка, есть ли юзер в бд
				isTrustedUser = mapOfAllUsersInDatabase[update.Message.Chat.ID][newUserId.ID]
				log.Printf("check in map finished. id is %s result is %s", newUserId.ID, isTrustedUser)
				log.Printf("check in map finished. result is %s", &isTrustedUser)
// потенциальная ошибка, будет false и ошибка записи в бд, если юзер уже есть, но антраст. пофиксить.
				if isTrustedUser == false{
					log.Printf("add to chat %s new userid %s", update.Message.Chat.ID, newUserId.ID)
					result, err := ignatDB.Exec(addUntrustedQuery, update.Message.Chat.ID, newUserId.ID)
					if err != nil {
						log.Fatal(err)
					}

					if mapOfAllUsersInDatabase[update.Message.Chat.ID] == nil{
						mapOfAllUsersInDatabase[update.Message.Chat.ID] = make(map[int]bool)
					}
					mapOfAllUsersInDatabase[update.Message.Chat.ID][newUserId.ID] = isTrustedUser

					log.Println("theoretically new user added")
					log.Println(result.RowsAffected())  // количество обновленных строк
					log.Println(mapOfAllUsersInDatabase)

				}
				log.Println("check finished")
			}
		}
	}
}
