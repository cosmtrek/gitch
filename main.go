package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/libgit2/git2go"
	"gopkg.in/urfave/cli.v1"
)

type User struct {
	Name  string
	Email string
}

type Commit struct {
	Id          string
	Author      User
	CreatedAt   time.Time
	Committer   User
	CommittedAt time.Time
	Message     string
}

type UserCommitResult struct {
	result []UserCommit
}

var (
	commitChan       chan *Commit
	commitResultChan chan UserCommitResult
)

func initApp() *cli.App {
	app := cli.NewApp()
	app.Name = "gitch"
	app.Usage = "g(b)itch analyses history of a git project"
	app.UsageText = "Please run at the project's root directory"
	app.Version = "0.1.0"
	app.Author = "Rick Yu <cosmtrek@gmail.com>"

	app.Commands = []cli.Command{
		{
			Name:    "authors",
			Aliases: []string{"au"},
			Usage:   "analyses contributors' work",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "order, o",
					Value: "count",
					Usage: "authors order by commit number",
				},
			},
			Action: func(c *cli.Context) error {
				authorsAction(c.String("order"))
				return nil
			},
		},
	}

	return app
}

func main() {
	app := initApp()
	app.Run(os.Args)
}

func authorsAction(order string) {
	cur, _ := os.Getwd()
	repo, err := git.OpenRepository(cur)
	if err != nil {
		log.Fatal("Failed to open repo, err: ", err)
	}
	defer repo.Free()

	commitChan = make(chan *Commit, 1000)
	commitResultChan = make(chan UserCommitResult)

	go traverseRepo(repo)
	go calculateCommits()

	for r := range commitResultChan {
		if order == "span" {
			sort.Sort(ByCommitSpan(r.result))
		} else {
			sort.Sort(ByCommitCount(r.result))
		}
		for _, v := range r.result {
			fmt.Printf("%s\n", v)
		}
	}
}

func traverseRepo(repo *git.Repository) {
	odb, err := repo.Odb()
	if err != nil {
		log.Fatal(err)
	}
	defer odb.Free()

	err = odb.ForEach(func(oid *git.Oid) error {
		obj, err := repo.Lookup(oid)
		if err != nil {
			return err
		}

		switch obj.Type().String() {
		case "Commit":
			c, _ := obj.AsCommit()
			commitChan <- &Commit{
				Id: c.Id().String(),
				Author: User{
					Name:  c.Author().Name,
					Email: c.Author().Email,
				},
				CreatedAt: c.Author().When.UTC(),
				Committer: User{
					Name:  c.Committer().Name,
					Email: c.Committer().Email,
				},
				CommittedAt: c.Committer().When.UTC(),
				Message:     c.Message(),
			}
			obj.Free()
		}
		return nil
	})
	if err != nil {
		log.Fatalln("Something wrong...")
	}
	close(commitChan)
}

type UserCommit struct {
	User
	CommitCount int
	CommitStart time.Time
	CommitEnd   time.Time
	CommitSpan  time.Duration
}

func (uc UserCommit) String() string {
	cs := fmt.Sprintf("%d-%d-%d", uc.CommitStart.Year(), uc.CommitStart.Month(), uc.CommitStart.Day())
	ce := fmt.Sprintf("%d-%d-%d", uc.CommitEnd.Year(), uc.CommitEnd.Month(), uc.CommitEnd.Day())
	return fmt.Sprintf("%s(%s), %d, %s(%s ~ %s)", uc.Name, uc.Email, uc.CommitCount, humanDuraion(uc.CommitSpan), cs, ce)
}

func humanDuraion(t time.Duration) string {
	m := t.Minutes()
	h := t.Hours()

	if h < 24 {
		return t.String()
	} else {
		d := int(h / 24)
		hh := int(h) % 24
		mm := int(m) % 60
		return fmt.Sprintf("%dd%dh%dm", d, hh, mm)
	}
}

type ByCommitCount []UserCommit

func (a ByCommitCount) Len() int           { return len(a) }
func (a ByCommitCount) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByCommitCount) Less(i, j int) bool { return a[i].CommitCount < a[j].CommitCount }

type ByCommitSpan []UserCommit

func (a ByCommitSpan) Len() int      { return len(a) }
func (a ByCommitSpan) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByCommitSpan) Less(i, j int) bool {
	return a[i].CommitSpan.Nanoseconds() < a[j].CommitSpan.Nanoseconds()
}

func calculateCommits() {
	commitHash := make(map[string]*UserCommit, 1000)

	for c := range commitChan {
		val, ok := commitHash[c.Author.Email]
		if ok {
			val.CommitCount += 1
			if c.CreatedAt.Before(val.CommitStart) {
				val.CommitStart = c.CreatedAt
			} else if c.CreatedAt.After(val.CommitEnd) {
				val.CommitEnd = c.CreatedAt
			}
			val.CommitSpan = val.CommitEnd.Sub(val.CommitStart)
		} else {
			commitHash[c.Author.Email] = &UserCommit{
				User: User{
					Name:  c.Author.Name,
					Email: c.Author.Email,
				},
				CommitCount: 1,
				CommitStart: c.CreatedAt,
				CommitEnd:   c.CreatedAt,
				CommitSpan:  1,
			}
		}
	}

	var result []UserCommit
	for _, v := range commitHash {
		result = append(result, *v)
	}

	commitResultChan <- UserCommitResult{
		result: result,
	}
	close(commitResultChan)
}
