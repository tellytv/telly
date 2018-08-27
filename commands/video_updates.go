package commands

import (
	"github.com/tellytv/telly/context"
)

// FireVideoUpdatesCommand Command to fire one off video source updates
func FireVideoUpdatesCommand() {
	cc, err := context.NewCContext()
	if err != nil {
		log.Fatalln("Couldn't create context", err)
	}
	if err = fireVideoUpdates(cc); err != nil {
		log.Errorln("Could not complete video updates " + err.Error())
	}
}

//
func fireVideoUpdates(cc *context.CContext) error {
	return nil
}

// StartFireVideoUpdates Scheduler triggered function to update video sources
func StartFireVideoUpdates(cc *context.CContext) {
	err := fireVideoUpdates(cc)
	if err != nil {
		log.Errorln("Could not complete video updates " + err.Error())
	}

	log.Infoln("Video source has been updated successfully")
}
