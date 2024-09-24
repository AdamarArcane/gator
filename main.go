package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/adamararcane/Gator/internal/config"
	"github.com/adamararcane/Gator/internal/database"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func main() {
	// Step 1: Read the config
	cfgFile, err := config.Read()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading config: %v\n", err)
		os.Exit(1)
	}

	// Step 2: Open a database connection
	db, err := sql.Open("postgres", cfgFile.Db_url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}

	// Step 3: Create database queries instance
	dbQueries := database.New(db)

	// Step 4: Create application state
	appState := &state{cfg: cfgFile, db: dbQueries}

	// Step 5: Define commands and their handlers
	cmds := commands{command: make(map[string]func(*state, command) error)}
	// Replace `handlerRegister` with your actual handler function
	cmds.register("register", handlerRegister)
	cmds.register("login", handlerLogin)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerGetUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", handlerAddFeed)

	// Step 6: Check and parse command-line arguments
	if len(os.Args) < 2 {
		fmt.Println("error: not enough arguments")
		os.Exit(1)
	}

	// Step 7: Execute given command
	name := os.Args[1]
	args := os.Args[2:]
	cmd := command{name, args}

	if err := cmds.run(appState, cmd); err != nil {
		fmt.Fprintf(os.Stderr, "error executing command: %v\n", err)
		os.Exit(1)
	}
}

type state struct {
	cfg config.Config
	db  *database.Queries
}

type command struct {
	name string
	args []string
}

func handlerLogin(appState *state, cmd command) error {
	// Step 1: Ensure a name argument was provided
	if len(cmd.args) < 1 {
		return fmt.Errorf("error: no username provided")
	}

	username := cmd.args[0]

	// Step 2: Check if the user exists in the database
	user, err := appState.db.GetUser(context.Background(), username)
	if err != nil {
		if err == sql.ErrNoRows {
			// Error handling if the user doesn't exist
			fmt.Fprintf(os.Stderr, "error: username '%s' does not exist\n", username)
			os.Exit(1)
		}
		return fmt.Errorf("error retrieving user: %v", err)
	}

	// Step 3: Update the configuration to set the logged-in user
	appState.cfg.SetUser(username)

	// Step 4: Provide user feedback for successful login
	fmt.Printf("User '%s' logged in successfully\n", user.Name)

	return nil
}

func handlerRegister(appState *state, cmd command) error {
	// Step 1: Ensure a name argument was provided
	if len(cmd.args) < 1 {
		return fmt.Errorf("error: no username provided")
	}

	username := cmd.args[0]

	// Step 2: Generate a new UUID for the user
	userID := uuid.New()

	// Step 3: Get the current time for timestamps
	now := time.Now()

	// Step 4: Attempt to create the user in the database
	user, err := appState.db.CreateUser(context.Background(), database.CreateUserParams{
		ID:        userID,
		CreatedAt: now,
		UpdatedAt: now,
		Name:      username,
	})
	if err != nil {
		// Check if the error is due to a duplicate username and handle appropriately
		return fmt.Errorf("error creating user: %v", err)
	}

	// Step 5: Update the config with the new user and handle any errors
	appState.cfg.SetUser(username)

	// Step 6: Print success message and debug information
	fmt.Printf("User created successfully: %v\n", user)

	return nil
}

func handlerReset(appState *state, cmd command) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("error: no args needed")
	}

	err := appState.db.ResetDatabase(context.Background())
	if err != nil {
		return fmt.Errorf("error reseting database")
	}

	return nil
}

func handlerGetUsers(appState *state, cmd command) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("error: no args needed")
	}

	users, err := appState.db.GetUsers(context.Background())
	if err != nil {
		return fmt.Errorf("error geting users from database")
	}

	for _, user := range users {
		if user == appState.cfg.Current_user_name {
			fmt.Printf("* %s (current)\n", user)
		} else {
			fmt.Printf("* %s\n", user)
		}

	}

	return nil
}

func handlerAgg(appState *state, cmd command) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("error: no args needed")
	}

	feed, err := fetchFeed("https://www.wagslane.dev/index.xml")
	if err != nil {
		return fmt.Errorf("error fetching feed: %w", err)
	}

	fmt.Println(feed)

	return nil

}

func handlerAddFeed(appState *state, cmd command) error {
	// Step 1: Ensure a name argument was provided
	if len(cmd.args) < 2 {
		return fmt.Errorf("error: not enough arguments (2)")
	}

	feedName := cmd.args[0]
	feedUrl := cmd.args[1]
	feedID := uuid.New()
	now := time.Now()
	user, err := appState.db.GetUser(context.Background(), appState.cfg.Current_user_name)
	if err != nil {
		return fmt.Errorf("error getting current user uuid")
	}

	feedRecord, err := appState.db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID:        feedID,
		CreatedAt: now,
		UpdatedAt: now,
		Name:      feedName,
		Url:       feedUrl,
		UserID:    user.ID,
	})
	if err != nil {
		return fmt.Errorf("error creating feed")
	}

	fmt.Println(feedRecord)

	return nil
}

type commands struct {
	command map[string]func(*state, command) error
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.command[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	if handler, exists := c.command[cmd.name]; exists {
		return handler(s, cmd)
	}
	return fmt.Errorf("command '%s' not found", cmd.name)
}

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func fetchFeed(feedURL string) (*RSSFeed, error) {

	var feed RSSFeed

	req, err := http.NewRequest("GET", feedURL, nil)
	if err != nil {
		return &RSSFeed{}, fmt.Errorf("error creating request")
	}
	req.Header.Add("User-Agent", "Gator")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &RSSFeed{}, fmt.Errorf("error getting rss feed")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &RSSFeed{}, fmt.Errorf("error reading response body")
	}

	err = xml.Unmarshal(body, &feed)
	if err != nil {
		return &RSSFeed{}, fmt.Errorf("error unmarshaling xml")
	}

	feed.Channel.Title = html.UnescapeString(feed.Channel.Title)
	feed.Channel.Description = html.UnescapeString(feed.Channel.Description)

	for i := range feed.Channel.Item {
		feed.Channel.Item[i].Title = html.UnescapeString(feed.Channel.Item[i].Title)
		feed.Channel.Item[i].Description = html.UnescapeString(feed.Channel.Item[i].Description)
	}

	return &feed, nil

}
