package uthoerrors

import (
	"fmt"
	"time"
)

type ErrorMessage string

const LBUnavailable ErrorMessage = "Sorry but due to some network resources unvaiable on this location we unable to deploy your cloud, Please come back after sometime"

func CheckMessage(msg ErrorMessage) (time.Duration, error) {
	switch msg {
	case LBUnavailable:
		return 30 * time.Second, nil
	default:
		return 5 * time.Second, fmt.Errorf(string(msg))
	}
}
