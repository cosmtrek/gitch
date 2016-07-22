package main

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/libgit2/git2go"
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

func main() {
	repo, err := git.OpenRepository("/Users/cosmtrek/Code/discourse")
	if err != nil {
		log.Fatal("Failed to open repo, err:", err)
	}

	commitChan = make(chan *Commit, 1000)
	commitResultChan = make(chan UserCommitResult)

	go traverseRepo(repo)
	go calculateCommits()

LOOP:
	for {
		select {
		case r := <-commitResultChan:
			for _, v := range r.result {
				fmt.Printf("%s\n", v)
			}
			break LOOP
		default:
		}
	}
}

func traverseRepo(repo *git.Repository) {
	log.Println("Traversing repo...")

	odb, err := repo.Odb()
	if err != nil {
		log.Fatal(err)
	}

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
		}
		return nil
	})
	if err != nil {
		log.Fatalln("Something wrong...")
	}
	close(commitChan)
	log.Println("Finishing traversing repo")
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
	return fmt.Sprintf("%s(%s), %d, %s(%s ~ %s)", uc.Name, uc.Email, uc.CommitCount, uc.CommitSpan, cs, ce)
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
	log.Println("Calculating commits...")
	commitHash := make(map[string]*UserCommit, 1000)

LOOP:
	for {
		select {
		case c := <-commitChan:
			if c == nil { // channel closed
				break LOOP
			}

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
	}

	var result []UserCommit
	for _, v := range commitHash {
		result = append(result, *v)
	}
	sort.Sort(ByCommitCount(result))

	commitResultChan <- UserCommitResult{
		result: result,
	}
	close(commitResultChan)
	log.Println("Finish calculatting")
}
