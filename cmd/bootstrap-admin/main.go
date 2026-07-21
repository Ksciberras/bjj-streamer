package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kyransciberras/bjj-streaming/internal/auth"
	"github.com/kyransciberras/bjj-streaming/internal/config"
	"github.com/kyransciberras/bjj-streaming/internal/database"
	"golang.org/x/term"
)

func main() {
	emailFlag := flag.String("email", "", "initial administrator email address")
	flag.Parse()
	email := strings.TrimSpace(*emailFlag)
	if email == "" {
		fmt.Fprint(os.Stderr, "Email: ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fail(err)
		}
		email = strings.TrimSpace(line)
	}
	if !auth.ValidEmail(email) {
		fail(fmt.Errorf("invalid email address"))
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fail(fmt.Errorf("password input requires an interactive terminal"))
	}
	fmt.Fprint(os.Stderr, "Password (minimum 12 characters): ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fail(err)
	}
	fmt.Fprint(os.Stderr, "Confirm password: ")
	confirmation, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fail(err)
	}
	if string(password) != string(confirmation) {
		fail(fmt.Errorf("passwords do not match"))
	}
	hash, err := auth.HashPassword(string(password))
	if err != nil {
		fail(err)
	}
	for i := range password {
		password[i] = 0
	}
	for i := range confirmation {
		confirmation[i] = 0
	}
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBConnectTimeout)
	defer cancel()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fail(err)
	}
	defer db.Close()
	user, err := auth.NewStore(db).BootstrapAdmin(ctx, email, hash)
	if err != nil {
		fail(err)
	}
	fmt.Fprintf(os.Stdout, "Created initial administrator %s (%s)\n", user.Email, user.ID)
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
