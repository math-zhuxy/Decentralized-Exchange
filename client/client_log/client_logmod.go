package client_log

import (
	"blockEmulator/params"
	"io"
	"log"
	"os"
)

type ClientLog struct {
	Clog *log.Logger
}

func NewClientLog() *ClientLog {
	writer1 := os.Stdout

	dirpath := params.LogWrite_path
	err := os.MkdirAll(dirpath, os.ModePerm)
	if err != nil {
		log.Panic(err)
	}
	writer2, err := os.OpenFile(dirpath+"/Client.log", os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		log.Panic(err)
	}
	pl := log.New(io.MultiWriter(writer1, writer2), "Client: ", log.Lshortfile|log.Ldate|log.Ltime)
	return &ClientLog{
		Clog: pl,
	}
}
