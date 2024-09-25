package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/adamararcane/gator/internal/config"
	"github.com/adamararcane/gator/internal/database"
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
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerGetFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("browse", middlewareLoggedIn(handlerBrowse))
	cmds.register("help", handlerHelp)

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

	fmt.Println("Gator has been reset")

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
	if len(cmd.args) < 1 || len(cmd.args) > 2 {
		return fmt.Errorf("error: input time duration (ex. 1s, 1m, 1hr)")
	}

	timeDur, err := time.ParseDuration(cmd.args[0])
	if err != nil {
		return fmt.Errorf("error parsing time time duration (ex. 1s, 1m, 1hr): %w", err)
	}

	fmt.Printf("Collecting feeds every %s...\n", timeDur)

	ticker := time.NewTicker(timeDur)
	for ; ; <-ticker.C {
		scrapeFeeds(appState)
	}

}

func handlerAddFeed(appState *state, cmd command, user database.User) error {
	// Step 1: Ensure a name argument was provided
	if len(cmd.args) < 2 {
		return fmt.Errorf("error: not enough arguments (2)")
	}

	feedName := cmd.args[0]
	feedUrl := cmd.args[1]
	feedID := uuid.New()
	now := time.Now()

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

	feedFollowParams := database.CreateFeedFollowParams{
		ID:     uuid.New(),
		UserID: user.ID,
		FeedID: feedID,
	}

	_, err = appState.db.CreateFeedFollow(context.Background(), feedFollowParams)
	if err != nil {
		return fmt.Errorf("error creating feed follow: %w", err)
	}

	fmt.Println(feedRecord)

	return nil
}

func handlerGetFeeds(appState *state, cmd command) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("error: no args needed")
	}

	feeds, err := appState.db.GetFeeds(context.Background())
	if err != nil {
		return fmt.Errorf("error geting users from database")
	}

	for _, feed := range feeds {
		userName, err := appState.db.GetUserName(context.Background(), feed.UserID)
		if err != nil {
			return fmt.Errorf("error matching UUIDs")
		}
		fmt.Printf("* %s\n", feed.Name)
		fmt.Printf("* %s\n", feed.Url)
		fmt.Printf("* %s\n", userName)
	}
	return nil
}

func handlerFollow(appState *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("error: url needed")
	}

	feed_follow_id := uuid.New()

	feed, err := appState.db.GetFeed(context.Background(), cmd.args[0])
	if err != nil {
		return fmt.Errorf("error getting url feed id")
	}

	feedFollowParams := database.CreateFeedFollowParams{
		ID:     feed_follow_id,
		UserID: user.ID,
		FeedID: feed.ID,
	}

	feedFollow, err := appState.db.CreateFeedFollow(context.Background(), feedFollowParams)
	if err != nil {
		return fmt.Errorf("error creating feed follow: %w", err)
	}

	fmt.Printf("%s followed %s\n", feedFollow[0].UserName, feedFollow[0].FeedName)
	return nil
}

func handlerFollowing(appState *state, cmd command, user database.User) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("error: no args needed")
	}

	userFollowing, err := appState.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return fmt.Errorf("error getting users follwed feeds: %w", err)
	}

	for _, name := range userFollowing {
		fmt.Printf("* %s\n", name.Name)
	}

	return nil

}

func handlerUnfollow(appState *state, cmd command, user database.User) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("error: url needed")
	}

	for _, arg := range cmd.args {
		feed, err := appState.db.GetFeed(context.Background(), arg)
		if err != nil {
			return fmt.Errorf("error getting url feed id: %w", err)
		}

		unfollowFeedParams := database.UnfollowFeedParams{
			FeedID: feed.ID,
			UserID: user.ID,
		}

		err = appState.db.UnfollowFeed(context.Background(), unfollowFeedParams)
		if err != nil {
			return fmt.Errorf("error unfollowing feed: %w", err)
		}

		fmt.Printf("* Unfollowed %s", feed.Name)
	}

	return nil
}

func handlerBrowse(s *state, cmd command, user database.User) error {
	limit := 2
	if len(cmd.args) == 1 {
		if specifiedLimit, err := strconv.Atoi(cmd.args[0]); err == nil {
			limit = specifiedLimit
		} else {
			return fmt.Errorf("invalid limit: %w", err)
		}
	}

	posts, err := s.db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
		UserID: user.ID,
		Limit:  int32(limit),
	})
	if err != nil {
		return fmt.Errorf("couldn't get posts for user: %w", err)
	}

	fmt.Printf("Found %d posts for user %s:\n", len(posts), user.Name)
	for _, post := range posts {
		fmt.Printf("%s from %s\n", post.PublishedAt.Time.Format("Mon Jan 2"), post.FeedName)
		fmt.Printf("--- %s ---\n", post.Title)
		fmt.Printf("    %v\n", post.Description.String)
		fmt.Printf("Link: %s\n", post.Url)
		fmt.Println("=====================================")
	}

	return nil
}

func handlerHelp(s *state, cmd command) error {
	descriptions := map[string]string{
		"help":      "Show available commands",
		"reset":     "Reset the application state",
		"register":  "Register a new user and log them in",
		"login":     "Log in as an existing user",
		"users":     "List all users",
		"addfeed":   "Add a new feed (requires login)",
		"feeds":     "List all feeds",
		"follow":    "Follow a feed (requires login)",
		"following": "List feeds you are following (requires login)",
		"unfollow":  "Unfollow a feed (requires login)",
		"agg":       "Aggregate data",
		"browse":    "Browse posts from your feeds (requires login)",
	}

	fmt.Println("Usage: Gator <command> <args>")
	fmt.Println("================================")
	fmt.Println("Available commands:")
	for name, description := range descriptions {
		fmt.Printf("  %-10s - %s\n", name, description)
	}
	return nil
}

// ===== Helper Functions =====

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.cfg.Current_user_name)
		if err != nil {
			return fmt.Errorf("error logging in user: %w", err)
		}
		return handler(s, cmd, user)
	}
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

func scrapeFeeds(appState *state) error {
	nextFeed, err := appState.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return fmt.Errorf("error getting next feed: %w", err)
	}

	err = appState.db.MarkFeedFetched(context.Background(), nextFeed.ID)
	if err != nil {
		return fmt.Errorf("error marking feed as fetched: %w", err)
	}

	feedData, err := fetchFeed(nextFeed.Url)
	if err != nil {
		return fmt.Errorf("error fetching feed: %w", err)
	}

	for _, feedItem := range feedData.Channel.Item {
		publishedAt := sql.NullTime{}
		if t, err := time.Parse(time.RFC1123Z, feedItem.PubDate); err == nil {
			publishedAt = sql.NullTime{
				Time:  t,
				Valid: true,
			}
		}
		createPostParams := database.CreatePostParams{
			ID:    uuid.New(),
			Title: feedItem.Title,
			Url:   feedItem.Link,
			Description: sql.NullString{
				String: feedItem.Description,
				Valid:  true,
			},
			PublishedAt: publishedAt,
			FeedID:      nextFeed.ID,
		}

		_, err = appState.db.CreatePost(context.Background(), createPostParams)
		if err != nil {
			if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
				continue
			}
			log.Printf("Couldn't create post: %v", err)
			continue
		}

	}

	log.Printf("Feed %s collected, %v posts found", nextFeed.Name, len(feedData.Channel.Item))
	return nil
}
