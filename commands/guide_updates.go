package commands

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/tellytv/telly/context"
	ginprometheus "github.com/zsais/go-gin-prometheus"
)

var (
	log = &logrus.Logger{
		Out: os.Stderr,
		Formatter: &logrus.TextFormatter{
			FullTimestamp: true,
		},
		Hooks: make(logrus.LevelHooks),
		Level: logrus.DebugLevel,
	}

	prom = ginprometheus.NewPrometheus("http")
)

// FireGuideUpdatesCommand Command to fire one off video source updates
func FireGuideUpdatesCommand() {
	cc, err := context.NewCContext()
	if err != nil {
		log.Fatalln("Couldn't create context", err)
	}
	if err = fireGuideUpdates(cc); err != nil {
		log.Errorln("Could not complete guide updates " + err.Error())
	}
}

func fireGuideUpdates(cc *context.CContext) error {

	return nil
}

// StartFireGuideUpdates Scheduler triggered function to update guide sources
func StartFireGuideUpdates(cc *context.CContext) {
	err := fireVideoUpdates(cc)
	if err != nil {
		log.Errorln("Could not complete video updates " + err.Error())
	}

	log.Infoln("Video source has been updated successfully")
}
