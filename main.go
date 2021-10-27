//Сделал Александр Никитин
//Created by Aleksandr Nikitin
//Skype: Travianbot
//Licence: Do not delete information about author.

package main

import (
	//"fmt"
	//"bytes"
	"github.com/DusanKasan/parsemail"
	"github.com/joho/godotenv"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	//"io/ioutil"
	"log"
	"os"
	"time"

	imap "github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

const EmailsCount = 100
const Filename_Settings = "settings.txt"

var myEnv map[string]string

//Email message constructed
type MessageStruct struct {
	From, Subject, Body string
}

var EmailClient *client.Client

func DownloadEmails(from int) int {
	//var Messages []MessageStruct
	MessageId := from - 1

	OutputDirectory := myEnv["OutputDirectory"]
	if OutputDirectory == "" {
		OutputDirectory = "Files"
		_ = os.Mkdir(OutputDirectory, os.ModePerm)
	}

	if strings.HasSuffix(OutputDirectory, "\\") == false {
		OutputDirectory = OutputDirectory + "\\"
	}

	sDownloadFromDate := myEnv["DownloadFromDate"]
	layout := "2006-01-02 15:04:05"
	DownloadFromDate, err := time.Parse(layout, sDownloadFromDate)
	if err != nil {
		log.Fatal("Wrong date: " + sDownloadFromDate)
	}

	sLastEmailID := myEnv["LastEmailID"]
	LastEmailID, err := strconv.Atoi(sLastEmailID)
	if err != nil {
		log.Fatal("Wrong LastEmailID: " + sLastEmailID)
	}

	//from := LastEmailID
	to := from + EmailsCount

	MessageChan := make(chan *imap.Message, to)

	done := make(chan error, 1)

	seqset := new(imap.SeqSet)
	seqset.AddRange(uint32(from), uint32(to))

	log.Println("Fetching emails from number", seqset.String())
	section := imap.BodySectionName{}
	//section := imap.FetchEnvelope
	go func() {
		done <- EmailClient.Fetch(seqset, []imap.FetchItem{section.FetchItem(), imap.FetchEnvelope}, MessageChan)
		//done <- EmailClient.Fetch(seqset, []imap.FetchItem{section.FetchItem()}, MessageChan)
	}()
	if err := <-done; err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	for RawMessage := range MessageChan {
		MessageId = int(RawMessage.SeqNum)
		sMessageId := strconv.Itoa(MessageId)

		r := RawMessage.GetBody(&section)
		if r == nil {
			log.Fatal("Server didn't return message body id: " + sMessageId)
			os.Exit(1)
		}

		email, err := parsemail.Parse(r)
		if err != nil {
			log.Println("Can not parse email id: " + sMessageId + " error: " + err.Error())
			continue
			//os.Exit(1)
		}

		//MessageId, err := strconv.Atoi(sMessageId)
		//if err != nil {
		//	log.Println("Can not parse MessageId to int")
		//	return
		//}
		if MessageId <= LastEmailID {
			SaveEnv(sMessageId)
			continue
		}

		EmailDate := RawMessage.Envelope.Date
		if EmailDate.Before(DownloadFromDate) {
			SaveEnv(sMessageId)
			continue
		}

		for _, file1 := range email.Attachments {
			Filename := file1.Filename
			ext := filepath.Ext(Filename)
			ext = strings.ToLower(ext)
			if ext != ".xls" && ext != ".xlsx" {
				SaveEnv(sMessageId)
				continue
			}

			println(file1.Filename)
			massBytes, err := ioutil.ReadAll(file1.Data)
			if err != nil {
				log.Fatal("Can not read message id: " + sMessageId)
				os.Exit(1)
			}

			PersonalName := RawMessage.Envelope.From[0].PersonalName

			EmailFrom := "From(" + PersonalName + " (" + RawMessage.Envelope.From[0].MailboxName + "@" + RawMessage.Envelope.From[0].HostName + "))"
			ioutil.WriteFile(OutputDirectory+EmailFrom+"_"+Filename, massBytes, 0644)

		}
		SaveEnv(sMessageId)

	}

	return MessageId
}

func main() {

	//var wg sync.WaitGroup

	LoadEnv()

	sLastEmailID := myEnv["LastEmailID"]
	LastEmailID, err := strconv.Atoi(sLastEmailID)
	if err != nil {
		log.Fatal("Wrong LastEmailID: " + sLastEmailID)
	}

	sPauseSeconds := myEnv["PauseSeconds"]
	PauseSeconds, err := strconv.Atoi(sPauseSeconds)
	if err != nil {
		log.Fatal("Wrong LastEmailID: " + sPauseSeconds)
	}

	start := time.Now()
	EmailClient = LoginEmail()
	defer EmailClient.Logout()
	defer log.Println("Logging out")

	defer func() {
		//log.Printf("Read %v messages", len(Messages))
		elapsed := time.Since(start)
		log.Printf("Time taken %s", elapsed)
	}()

	from := LastEmailID + 1

	for {
		from = DownloadEmails(from)
		from = from + 1
		time.Sleep(time.Second * time.Duration(PauseSeconds))
	}

	//const EmailsPerBatch = 1
	//const TotalEmails = 1

}

func SaveEnv(sMessageId string) {
	myEnv["LastEmailID"] = sMessageId
	err := godotenv.Write(myEnv, Filename_Settings)
	if err != nil {
		log.Fatal("Can not write message id: " + sMessageId)
		os.Exit(1)
		//return
	}

}

func LoadEnv() {
	var err error
	//err := godotenv.Load(Filename_Settings)
	//if err != nil {
	//	log.Fatal("Error loading " + Filename_Settings + " file, error: " + err.Error())
	//}

	myEnv, err = godotenv.Read(Filename_Settings)
	if err != nil {
		log.Fatal("Error parse " + Filename_Settings + " file, error: " + err.Error())
	}

}

func LoginEmail() *client.Client {

	log.Println("Connecting to server...")

	// Connect to server
	EmailClient, err := client.DialTLS(myEnv["IMAP_SERVER"], nil)
	if err != nil {
		log.Fatal(err)
	}

	// Don't forget to logout
	//defer EmailClient.Logout()
	//defer log.Println("Logging out")

	// Login
	email := myEnv["EMAIL"]
	password := myEnv["PASSWORD"]
	if err := EmailClient.Login(email, password); err != nil {
		log.Fatal(err)
	}
	log.Println("Logged in")

	// Select INBOX
	_, err = EmailClient.Select("INBOX", false)
	if err != nil {
		log.Fatal(err)
	}
	//log.Println("number of messages : ", mbox.Messages)

	//seqset := new(imap.SeqSet)
	//seqset.AddRange(uint32(from), uint32(to))
	//
	//log.Println("Fetching emails from number", seqset.String())
	//err = EmailClient.Fetch(seqset, []imap.FetchItem{section.FetchItem()}, MessageChan)
	//if err != nil {
	//	log.Println(err)
	//}

	return EmailClient
}

func FetchEmail(MessageChan chan *imap.Message, from, to int, section *imap.BodySectionName) {
	done := make(chan error, 1)

	seqset := new(imap.SeqSet)
	seqset.AddRange(uint32(from), uint32(to))

	log.Println("Fetching emails from number", seqset.String())
	done <- EmailClient.Fetch(seqset, []imap.FetchItem{section.FetchItem()}, MessageChan)
	if err := <-done; err != nil {
		log.Fatal(err)
	}

}
