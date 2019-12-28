package main

import (
    "github.com/go-telegram-bot-api/telegram-bot-api"
    "log"
    "os"
    "time"
    "encoding/json"
	"database/sql"
	 _ "github.com/mattn/go-sqlite3"
	 "path/filepath"
	 "unicode"
)

type Config struct {
    TelegramBotToken string
}


func main() {
	const botTimeOutValue int = 60
	const configJSONFileName string = "config.json" 
	const logOutputFileName string = "ignat_logfile.log"
	const cooldowndForBannedUser int64 = 45
	const databaseFileName  string = "/db/ignat_db.db"
	const databaseDriverName string = "sqlite3"


	appAbsoluteDirName, err := filepath.Abs(filepath.Dir(os.Args[0])) 
	    if err != nil { 
	      log.Fatal(err) 
	    } 
	appAbsoluteDirName = appAbsoluteDirName + "/"

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	LogOutputFile, err := os.OpenFile(appAbsoluteDirName + logOutputFileName, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666) 
	    if err != nil { 
	    log.Fatalf("error opening file: %v", err) 
	    }

	defer LogOutputFile.Close()

	log.SetOutput(LogOutputFile) 


    configFile, _ := os.Open(appAbsoluteDirName + configJSONFileName)
    decoder := json.NewDecoder(configFile)
    configuration := Config{}
    err = decoder.Decode(&configuration)
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
	updateFromBot.Timeout = botTimeOutValue

	ignatDB, err := sql.Open(databaseDriverName, appAbsoluteDirName + databaseFileName)
	if err != nil {
		log.Fatal(err)
	}
	defer ignatDB.Close()

	var isTrustedUser, isExist bool
	var isLinkInMessage bool = false
	var rowsAffected int64

	addUntrustedQuery := "insert into ignated_chat_users (chat_id, user_id, is_trusted) values ($1, $2, 1)"
	addTrustedQuery := "insert into ignated_chat_users (chat_id, user_id) values ($1, $2)"
	updateTrustedQuery := "update ignated_chat_users set is_trusted = true where chat_id = $1 and user_id = $2"	
	getUsersQuery := "select chat_id, user_id, is_trusted from ignated_chat_users"
	deleteSpammerQuery := "delete from ignated_chat_users where chat_id = $1 and user_id = $2"

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

		log.Printf("from [%s][%d] was message.Text: %s", update.Message.From.UserName, update.Message.From.ID, update.Message.Text)
		log.Printf("from [%s][%d] was message.Caption: %s", update.Message.From.UserName, update.Message.From.ID, update.Message.Caption)
		log.Printf("from [%s][%d] was update.CallbackQuery: %s", update.Message.From.UserName, update.Message.From.ID, update.CallbackQuery)
		log.Printf("from [%s][%d] was update.InlineQuery: %s", update.Message.From.UserName, update.Message.From.ID, update.InlineQuery)

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
		isTrustedUser, isExist = mapOfAllUsersInDatabase[update.Message.Chat.ID][update.Message.From.ID]

		log.Printf("isTrustedUser = %t, isExist = %t", isTrustedUser, isExist)

		if isExist && isTrustedUser == false {
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
				log.Printf("we have a link from untrusted user %s %s %s", update.Message.From.FirstName, update.Message.From.LastName, update.Message.From.UserName)
				bot.DeleteMessage(tgbotapi.DeleteMessageConfig{update.Message.Chat.ID, update.Message.MessageID})
				log.Printf("message with link from %d was deleted ", update.Message.From.ID)
				bot.KickChatMember(tgbotapi.KickChatMemberConfig{tgbotapi.ChatMemberConfig{update.Message.Chat.ID, "", "", update.Message.From.ID}, time.Now().Unix() + cooldowndForBannedUser})  

				result, err := ignatDB.Exec(deleteSpammerQuery, update.Message.Chat.ID, update.Message.From.ID)
				if err != nil {
					log.Fatal(err)
				}
				rowsAffected, _ = result.RowsAffected()
				log.Printf("removed spammer from database. amount of affected rows is %d ", rowsAffected)
				delete(mapOfAllUsersInDatabase[update.Message.Chat.ID], update.Message.From.ID)
				log.Println(mapOfAllUsersInDatabase)


			} else{
				isTrustedUser = true
				log.Printf("We have a message with no link from untrusted user. Let's update the user as trusted")
				log.Printf("update in chat %d userid %d", update.Message.Chat.ID, update.Message.From.ID)
				result, err := ignatDB.Exec(updateTrustedQuery, update.Message.Chat.ID, update.Message.From.ID)
				if err != nil {
					log.Fatal(err)
				}
				rowsAffected, _ = result.RowsAffected()

				if mapOfAllUsersInDatabase[update.Message.Chat.ID] == nil{
					mapOfAllUsersInDatabase[update.Message.Chat.ID] = make(map[int]bool)
				}
				mapOfAllUsersInDatabase[update.Message.Chat.ID][update.Message.From.ID] = isTrustedUser

				log.Printf("theoretically user updated. amount of affected rows is %d", rowsAffected)
				log.Println(mapOfAllUsersInDatabase)

			}

		}else if isExist == false && update.Message.NewChatMembers ==nil {
			// we had a user in a chat before bot has been added
			// so we will add a user as trusted user
			isTrustedUser = true
			log.Printf("We have a message from user who is not in map. Let's add the user as trusted")
			log.Printf("add to chat %d new userid %d", update.Message.Chat.ID, update.Message.From.ID)
			result, err := ignatDB.Exec(addTrustedQuery, update.Message.Chat.ID, update.Message.From.ID)
			if err != nil {
				log.Fatal(err)
			}
			rowsAffected, _ = result.RowsAffected()

			if mapOfAllUsersInDatabase[update.Message.Chat.ID] == nil{
				mapOfAllUsersInDatabase[update.Message.Chat.ID] = make(map[int]bool)
			}
			mapOfAllUsersInDatabase[update.Message.Chat.ID][update.Message.From.ID] = isTrustedUser

			log.Printf("theoretically new user added as trusted. amount of affected rows is %d", rowsAffected)
			log.Println(mapOfAllUsersInDatabase)
		}

		if update.Message.NewChatMembers !=nil {
			for _, newUserId := range *update.Message.NewChatMembers {
//				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
//				msg.Text = "привет, кремлебот, " + newUserId.FirstName 
//				msg.ReplyToMessageID = update.Message.MessageID
//				bot.Send(msg)
				log.Printf("we have a new user. Let's check for chineese symbols")
				var strForCheck = ""
				strForCheck = newUserId.FirstName + newUserId.LastName + newUserId.UserName
				for _, charValue := range strForCheck {
					if unicode.Is(unicode.Han, charValue) {
				    	log.Printf("%s has a Chinese character. Removed.", string(strForCheck))
						bot.KickChatMember(tgbotapi.KickChatMemberConfig{tgbotapi.ChatMemberConfig{update.Message.Chat.ID, "", "", update.Message.From.ID}, time.Now().Unix() + cooldowndForBannedUser})  
						break
				    } 
				}
				
				log.Printf("we have a new user. Do check if userid is in the db")
				log.Printf("Before isTrustedUser = %t, isExist = %t", isTrustedUser, isExist)

				isTrustedUser, isExist = mapOfAllUsersInDatabase[update.Message.Chat.ID][newUserId.ID]

				log.Printf("After isTrustedUser = %t, isExist = %t", isTrustedUser, isExist)
				if isExist == false{
					log.Printf("add to chat %d new userid %d", update.Message.Chat.ID, newUserId.ID)
					result, err := ignatDB.Exec(addUntrustedQuery, update.Message.Chat.ID, newUserId.ID)
					if err != nil {
						log.Fatal(err)
					}
					rowsAffected, _ = result.RowsAffected()

					if mapOfAllUsersInDatabase[update.Message.Chat.ID] == nil{
						mapOfAllUsersInDatabase[update.Message.Chat.ID] = make(map[int]bool)
					}
					mapOfAllUsersInDatabase[update.Message.Chat.ID][newUserId.ID] = isTrustedUser

					log.Printf("theoretically new user added as untrusted. amount of affected rows is %d", rowsAffected)
					log.Println(mapOfAllUsersInDatabase)

					}				
			}
		}
	}
}
