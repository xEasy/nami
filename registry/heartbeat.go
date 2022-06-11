package registry

import (
	"fmt"
	"net/http"
	"time"
)

func Heartbeat(regiestry, addr string, duration time.Duration) {
	if duration == 0 {
		duration = defaultTimeout - time.Second*60
	}
	var err error
	err = sendHeartbeat(regiestry, addr)
	if err != nil {
		go func(regiestry, addr string) {
			ticker := time.NewTicker(duration)
			var err error
			for err == nil {
				<-ticker.C
				err = sendHeartbeat(regiestry, addr)
			}
		}(regiestry, addr)
	}
}

func sendHeartbeat(regiestry, addr string) error {
	fmt.Println("rpc server: ", addr, " sending heartbeat to regiestry ", regiestry)
	client := &http.Client{}
	req, _ := http.NewRequest("POST", regiestry, nil)
	req.Header.Set("X-Namirpc-Server", addr)
	if _, err := client.Do(req); err != nil {
		fmt.Println("rpc server: Heartbeat error ", err)
		return err
	}
	return nil
}
