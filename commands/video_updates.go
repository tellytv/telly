package commands

import (
	"fmt"

	"github.com/tellytv/telly/context"
)

// FireVideoUpdatesCommand Command to fire one off video source updates
func FireVideoUpdatesCommand() {
	cc, err := context.NewCContext()
	if err != nil {
		panic(fmt.Errorf("couldn't create context: %s", err))
	}
	if err = fireVideoUpdates(cc); err != nil {
		panic(fmt.Errorf("could not complete video updates: %s", err))
	}
}

func fireVideoUpdates(cc *context.CContext) error {
	fmt.Println("VIDEO source update is beginning")
	return nil
}

// StartFireVideoUpdates Scheduler triggered function to update video sources
func StartFireVideoUpdates(cc *context.CContext, providerID int) {
	err := fireVideoUpdates(cc)
	if err != nil {
		panic(fmt.Errorf("could not complete video updates: %s", err.Error()))
	}

	fmt.Println("Video source has been updated successfully")
}
