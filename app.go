package main

import (
	"flag"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

var log *zap.Logger

const DEFAULT_USER_ID = 1

func init() {
	log = BuildLogger("pc")
}

func main() {
	importOpml := flag.String("import", "", "Path to the OPML file to import")
	checkUpdates := flag.Bool("update", false, "Check for new podcast episodes")
	download := flag.Bool("download", false, "Download podcasts after import or update")
	flag.Parse()

	db := NewDB()

	if *importOpml == "" && !*checkUpdates && !*download {
		fmt.Println("You must provide one of the -import or -update or -download flags")
		os.Exit(1)
	}

	if *importOpml != "" {
		opml, err := ParseOpml(*importOpml)
		if err != nil {
			log.Sugar().Fatal("could not parse OPML file: %v", err)
		}

		err = db.InsertPodcastsToDB(DEFAULT_USER_ID, opml.Body)
		if err != nil {
			log.Sugar().Fatalf("could not insert podcasts into database: %v", err)
		}
	}

	if *checkUpdates {
		CheckForNewEpisodes(db)
	}

	if *download {
		err := DownloadNewEpisodes(db)
		if err != nil {
			log.Sugar().Fatalf("could not download new episodes: %v", err)
		}
	}
}
