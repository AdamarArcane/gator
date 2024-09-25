# Gator CLI

Gator is a command-line interface (CLI) application for managing and interacting with your RSS feeds. This README will guide you through the installation process and help you get started with using Gator.
Prerequisites

Before you begin, ensure you have the following installed on your system:

* Go (version 1.16 or higher)
* PostgreSQL (version 12 or higher)

## Installation
### Install Go

If you haven't installed Go yet, download it from the [official website](https://go.dev/doc/install) and follow the installation instructions for your operating system.
Install PostgreSQL

Download and install [PostgreSQL](https://www.postgresql.org/download/) from the official website and follow the setup instructions for your operating system.
### Install the Gator CLI

To install the Gator CLI, use the `go install` command:

```
go install github.com/adamararcane/gator@latest
```

### Configuration

Gator requires a configuration file to connect to your PostgreSQL database and manage feeds.
Set Up the Config File

By default, Gator looks for a `.gatorconfig.json` file in your home directory:

* Unix/Linux/macOS: `$HOME/.gatorconfig.json`
* Windows: `%USERPROFILE%\.gatorconfig.json`

Create the config file in your home directory:
```
touch ~/.gatorconfig.json
```
### Configure the Database Connection

Edit the `.gatorconfig.json` file and add your database connection details:
```
{
  "db_url": "postgres://postgres:postgres@localhost:5432/gator?sslmode=disable",
  "current_user_name": "john"
}
```
* Replace username and password with your PostgreSQL credentials.
* Replace gatordb with the name of your database.

### Database Migrations

Gator uses [Goose](https://github.com/pressly/goose) for database migrations, and the migration files are included in the package.

## Running the Program

After setting up the configuration and database, you can start using Gator.
### Example Commands

Here are some commands you can run:
#### Register a New User
```
gator register "your_username"
```
#### Log In
```
gator login "your_username"
```
#### Add a New Feed
```
gator addfeed "TechCrunch" "https://techcrunch.com/feed/"
```
#### List All Feeds
```
gator feeds
```
#### Follow a Feed
```
gator follow "TechCrunch"
```
#### List Feeds You Are Following
```
gator following
```
#### Unfollow a Feed
```
gator unfollow --name "TechCrunch"
```
#### Browse Posts from Your Feeds
```
gator browse
```
#### Reset the Application State
```
gator reset
```
#### Help Command

Display help information about Gator commands
```
gator help
```

## License

This project is licensed under the MIT License.
