package main

import (
	"database/sql"
	"time"
)

type Podcast struct {
	Id       int
	Text     string `xml:"text,attr"`
	XMLUrl   string `xml:"xmlUrl,attr"`
	HTMLUrl  string `xml:"htmlUrl,attr"`
	ImageUrl string `xml:"imageUrl,attr"`
}

type Episode struct {
	Id          int
	PodcastName string
	URL         string
}

type DB struct {
	database *sql.DB
}

var localDB *DB

func init() {
	localDB = NewDB()
}

func CloseDB() {
	localDB.database.Close()
}

func NewDB() *DB {
	sqliteDB, err := sql.Open("sqlite3", AppConfig.DatabaseFile)
	if err != nil {
		log.Sugar().Fatalf("Could not open SQLite database: %v", err)
	}

	InitializeDatabase(sqliteDB)

	return &DB{
		database: sqliteDB,
	}
}

func InitializeDatabase(db *sql.DB) error {
	createPodcastTableQuery := `
	CREATE TABLE IF NOT EXISTS podcasts(
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		name TEXT NOT NULL,
		xml_url TEXT NOT NULL,
		html_url TEXT,
		image_url TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, name),
		FOREIGN KEY(user_id) REFERENCES users(id)
)
`

	_, err := db.Exec(createPodcastTableQuery)
	if err != nil {
		return err
	}

	createEpisodesTableQuery := `
		CREATE TABLE IF NOT EXISTS episodes(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			podcast_id INTEGER,
			url TEXT NOT NULL,
			file_name TEXT,
			downloaded INTEGER NOT NULL DEFAULT 0,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			update_time DATETIME,
			FOREIGN KEY(podcast_id) REFERENCES podcasts(id)
		)
	
	`

	_, err = db.Exec(createEpisodesTableQuery)
	if err != nil {
		return err
	}

	createEpisodesArchiveTableQuery := `
		CREATE TABLE IF NOT EXISTS archived_episodes(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			original_episode_id INTEGER,
			podcast_id INTEGER,
			url TEXT NOT NULL,
			downloaded INTEGER NOT NULL,
			timestamp DATETIME,
			FOREIGN KEY(original_episode_id) REFERENCES episodes(id)
		)
	
	`

	_, err = db.Exec(createEpisodesArchiveTableQuery)
	if err != nil {
		return err
	}

	createUsersTableQuery := `
		CREATE TABLE IF NOT EXISTS users(
			id INTEGER PRIMARY KEY,
			username TEXT NOT NULL,
			email TEXT NOT NULL,
			password TEXT NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	)
	`
	_, err = db.Exec(createUsersTableQuery)

	return err
}

func (db *DB) InsertPodcastsToDB(userID int, podcasts []Podcast) error {
	for _, podcast := range podcasts {
		// Check if the podcast already exists
		var existingID sql.NullInt64
		err := db.database.QueryRow("SELECT id FROM podcasts WHERE user_id = ? AND name = ?", userID, podcast.Text).Scan(&existingID)

		if err != nil && err != sql.ErrNoRows {
			return err
		}

		if err == sql.ErrNoRows { // Insert new podcast
			// Find the maximum id
			var maxID sql.NullInt64
			err = db.database.QueryRow("SELECT MAX(id) FROM podcasts").Scan(&maxID)
			if err != nil {
				return err
			}

			nextID := 1
			if maxID.Valid {
				nextID = int(maxID.Int64) + 1
			}

			_, err = db.database.Exec(`
				INSERT INTO podcasts(id, user_id, name, xml_url, html_url, image_url)
				VALUES (?, ?, ?, ?, ?, ?)`,
				nextID,
				userID,
				podcast.Text,
				podcast.XMLUrl,
				podcast.HTMLUrl,
				podcast.ImageUrl,
			)
		} else { // Update existing podcast
			_, err = db.database.Exec(`
				UPDATE podcasts SET xml_url = ?, html_url = ?, image_url = ? WHERE id = ?`,
				podcast.XMLUrl,
				podcast.HTMLUrl,
				podcast.ImageUrl,
				existingID.Int64,
			)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) GetPodcastsFromDB(userID int) ([]Podcast, error) {
	rows, err := db.database.Query("SELECT id, xml_url FROM podcasts WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	podcasts := make([]Podcast, 0)
	for rows.Next() {
		var p Podcast
		if err := rows.Scan(&p.Id, &p.XMLUrl); err != nil {
			return nil, err
		}
		podcasts = append(podcasts, p)
	}

	return podcasts, nil
}

func (db *DB) InsertEpisodesToDB(podcastId int, episodes []Item) (int, error) {
	newEpisodesCount := 0 // Counter for new episodes

	for _, item := range episodes {
		// Check if the episode already exists
		var existingEpisode struct {
			ID         int
			URL        string
			Downloaded int
		}
		err := db.database.QueryRow("SELECT id, url, downloaded FROM episodes WHERE podcast_id = ? AND url = ?", podcastId, item.Enclosure.Url).Scan(&existingEpisode.ID, &existingEpisode.URL, &existingEpisode.Downloaded)

		if err != nil && err != sql.ErrNoRows {
			return newEpisodesCount, err
		}

		if err == sql.ErrNoRows { // Insert new episode
			_, err = db.database.Exec(`
				INSERT INTO episodes(podcast_id, url)
				VALUES (?, ?)`,
				podcastId,
				item.Enclosure.Url,
			)
			if err == nil {
				newEpisodesCount++ // Increment the counter if a new episode was inserted
			}
		} else if existingEpisode.URL != item.Enclosure.Url {
			// URL has changed, update existing episode
			// Archive the existing episode
			_, err = db.database.Exec(`
				INSERT INTO archived_episodes(original_episode_id, podcast_id, url, downloaded, timestamp)
				VALUES (?, ?, ?, ?, ?)`,
				existingEpisode.ID,
				podcastId,
				existingEpisode.URL,
				existingEpisode.Downloaded,
				time.Now(),
			)
			if err != nil {
				return newEpisodesCount, err
			}

			// Update the current episode
			_, err = db.database.Exec(`
				UPDATE episodes SET url = ? WHERE id = ?`,
				item.Enclosure.Url,
				existingEpisode.ID,
			)
		}

		if err != nil {
			return newEpisodesCount, err
		}
	}

	return newEpisodesCount, nil
}

func (db *DB) MarkEpisodeAsDownloaded(episodeId int, fileName string) error {
	_, err := db.database.Exec(`
		UPDATE episodes 
		SET downloaded = 1, file_name = ? 
		WHERE id = ?`,
		fileName,
		episodeId,
	)

	log.Sugar().Debugf("*** id: %d file name: %s", episodeId, fileName)

	return err
}

func (db *DB) GetNewEpisodesFromDB() ([]Episode, error) {
	rows, err := db.database.Query(`
		SELECT e.id, p.name as podcast_name, e.url 
		FROM episodes e
		JOIN podcasts p ON e.podcast_id = p.id
		WHERE e.downloaded = 0
	`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	episodes := make([]Episode, 0)
	for rows.Next() {
		var episode Episode
		if err := rows.Scan(&episode.Id, &episode.PodcastName, &episode.URL); err != nil {
			return nil, err
		}
		episodes = append(episodes, episode)
	}

	return episodes, nil
}
