// Package main is the entry point for the WeChat subscription service.
package main

import (
	"go.uber.org/fx"

	fxmodules "git.uhomes.net/uhs-go/wechat-subscription-svc/internal/fx"
)

func main() {
	app := fx.New(fxmodules.AllModules)
	app.Run()
}
