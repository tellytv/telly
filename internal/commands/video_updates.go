package commands

import (
	"fmt"

	"github.com/tellytv/telly/internal/context"
	"github.com/tellytv/telly/internal/models"
)

// FireVideoUpdatesCommand Command to fire one off video source updates
func FireVideoUpdatesCommand() {
	cc, err := context.NewCContext()
	if err != nil {
		panic(fmt.Errorf("couldn't create context: %s", err))
	}
	if err = fireVideoUpdates(cc, nil); err != nil {
		panic(fmt.Errorf("could not complete video updates: %s", err))
	}
}

func fireVideoUpdates(cc *context.CContext, provider *models.VideoSource) error {
	fmt.Println("VIDEO source update is beginning")
	return nil
}

// StartFireVideoUpdates Scheduler triggered function to update video sources
func StartFireVideoUpdates(cc *context.CContext, provider *models.VideoSource) {
	err := fireVideoUpdates(cc, provider)
	if err != nil {
		panic(fmt.Errorf("could not complete video updates: %s", err.Error()))
	}

	fmt.Println("Video source has been updated successfully")
}
