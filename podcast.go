package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cavaliergopher/grab/v3"
	"go.uber.org/zap"
)

type Rss struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	XMLName     xml.Name `xml:"channel"`
	Title       string   `xml:"title"`
	Description string   `xml:"description"`
	Language    string   `xml:"language"`
	Item        []Item   `xml:"item"`
}

type Item struct {
	XMLName     xml.Name  `xml:"item"`
	Title       string    `xml:"title"`
	Description string    `xml:"description"`
	PubDate     string    `xml:"pubDate"`
	Link        string    `xml:"link"`
	Image       Image     `xml:"image"` // or `xml:"itunes:image"` depending on your RSS feed
	Enclosure   Enclosure `xml:"enclosure"`
}

type Image struct {
	XMLName xml.Name `xml:"image"` // or `xml:"itunes:image"` depending on your RSS feed
	Url     string   `xml:"url,attr"`
}

type Enclosure struct {
	XMLName xml.Name `xml:"enclosure"`
	Url     string   `xml:"url,attr"`
}

type Opml struct {
	XMLName xml.Name  `xml:"opml"`
	Body    []Podcast `xml:"body>outline"`
}

func ParseOpml(filename string) (*Opml, error) {
	xmlFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer xmlFile.Close()

	byteValue, _ := io.ReadAll(xmlFile)

	var opml Opml
	xml.Unmarshal(byteValue, &opml)

	return &opml, nil
}

func ParseRSSFeed(url string) (*Rss, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Sugar().Errorf("rss feed error: %s %s", url, resp.Status)
		return nil, fmt.Errorf("rss feed error: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rss Rss
	err = xml.Unmarshal(body, &rss)
	if err != nil {
		return nil, err
	}

	return &rss, nil
}

func CheckForNewEpisodes(db *DB) {
	podcasts, err := db.GetPodcastsFromDB(DEFAULT_USER_ID)
	if err != nil {
		log.Error("failed to select podcasts", zap.Error(err))
		return
	}

	newEpisodesCount := 0
	for _, p := range podcasts {
		rss, err := ParseRSSFeed(p.XMLUrl)
		if err != nil {
			log.Sugar().Errorf("failed to parse RSS feed for %s %s: %v", p.Text, p.XMLUrl, err)
			continue
		}

		// Update podcast info
		err = db.UpdatePodcastInfo(p.Id, rss.Channel.Title, rss.Channel.Description, rss.Channel.Language)
		if err != nil {
			log.Sugar().Error("failed to update podcast info in database", err)
			continue
		}

		// Convert the channel's items to episode format
		episodes := make([]Item, len(rss.Channel.Item))
		for i, item := range rss.Channel.Item {
			episodes[i] = Item{
				Title:       item.Title,
				Description: item.Description,
				PubDate:     item.PubDate,
				Link:        item.Link,
				Image:       item.Image,
				Enclosure:   item.Enclosure,
			}
		}

		// Insert new episodes
		newNum, err := db.InsertEpisodesToDB(p.Id, episodes)
		newEpisodesCount += newNum
		if err != nil {
			log.Sugar().Error("failed to insert episodes into database", err)
			continue
		}
	}
	log.Sugar().Infof("%d new episodes added", newEpisodesCount)
}

func DownloadNewEpisodes(db *DB) error {
	client := grab.NewClient()

	// Get the list of not downloaded episodes
	episodes, err := db.GetNewEpisodesFromDB()
	if err != nil {
		log.Sugar().Errorf("can't get new episoded from db: %v", err)
		return err
	}

	for _, episode := range episodes {
		podcastDir := filepath.Join(AppConfig.DownloadDir, episode.PodcastName)
		err = os.MkdirAll(podcastDir, 0755) // Ensure the podcast directory exists

		if err != nil {
			log.Sugar().Error("creating podcast directort failed: ", err)
			return err
		}

		fileName := fmt.Sprintf("%d_%s", episode.Id, filepath.Base(episode.URL))
		filePath := filepath.Join(podcastDir, fileName)

		req, err := grab.NewRequest(filePath, episode.URL)
		if err != nil {
			return err
		}

		resp := client.Do(req)
		select {
		case <-resp.Done:
			log.Sugar().Debugf("download done: %s", episode.URL)

			err := resp.Err()
			if err != nil {
				log.Sugar().Warn("download failed: %s %v ", episode.URL, err)
			} else {
				// Mark the episode as downloaded
				if err := db.MarkEpisodeAsDownloaded(episode.Id, fileName); err != nil {
					log.Sugar().Warn("failed to mark episode as downloaded: ", err)
				}
			}
		}
	}

	// Check each file was downloaded without error
	err = filepath.Walk(AppConfig.DownloadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Failed to access file %s: %v\n", path, err)
		}
		return nil
	})

	return err
}
