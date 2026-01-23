package main

import (
	"github.com/nanami9426/imgo/router"
	"github.com/nanami9426/imgo/utils"
)

func main() {
	utils.InitConfig()
	utils.InitMySQL()
	r := router.Router()
	r.Run(":8000")
}
