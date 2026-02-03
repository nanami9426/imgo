package main

import (
	"github.com/nanami9426/imgo/router"
	"github.com/nanami9426/imgo/utils"
)

func main() {
	utils.InitConfig()
	r := router.Router()
	r.Run(":8000")
}
