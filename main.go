package main

import (
	"fmt"
	"github.com/jroimartin/gocui"
	"github.com/sadovojav/onesecmail"
	"log"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

var mailbox *onesecmail.Mailbox

var (
	emails []*onesecmail.Mail

	done = make(chan struct{})
	wg   sync.WaitGroup

	mu  sync.Mutex // protects ctr
	ctr = 0
)

func main() {
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	g.Highlight = true
	g.SelFgColor = gocui.ColorGreen

	g.SetManagerFunc(layout)

	rand.Seed(time.Now().UnixNano())

	min := 6
	max := 12

	login := String(rand.Intn(max-min+1) + min)
	mailbox = onesecmail.NewMailbox(login, "", nil)

	g.Update(func(g *gocui.Gui) error {
		v, err := g.View("address")

		if err != nil {
			return err
		}

		v.Clear()

		fmt.Fprintf(v, "%s", mailbox.Address())

		return nil
	})

	if err := keybindings(g); err != nil {
		log.Panicln(err)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go receiveEmails(g)
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}

	wg.Wait()
}

func keybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("list", gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("list", gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("list", gocui.KeyEnter, gocui.ModNone, showMsg); err != nil {
		return err
	}

	if err := g.SetKeybinding("view", gocui.KeyArrowUp, gocui.ModNone,
		func(g *gocui.Gui, v *gocui.View) error {
			if err := scrollView(v, -1); err != nil {
				return err
			}
			return nil
		}); err != nil {
		return err
	}
	if err := g.SetKeybinding("view", gocui.KeyArrowDown, gocui.ModNone,
		func(g *gocui.Gui, v *gocui.View) error {
			if err := scrollView(v, 1); err != nil {
				return err
			}
			return nil
		}); err != nil {
		return err
	}

	if err := g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, nexView); err != nil {
		return err
	}

	return nil
}

func cursorDown(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy+1); err != nil {
			ox, oy := v.Origin()
			if err := v.SetOrigin(ox, oy+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func cursorUp(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		ox, oy := v.Origin()
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy-1); err != nil && oy > 0 {
			if err := v.SetOrigin(ox, oy-1); err != nil {
				return err
			}
		}
	}
	return nil
}

func showMsg(g *gocui.Gui, v *gocui.View) error {
	var l string
	var err error

	_, cy := v.Cursor()
	if l, err = v.Line(cy); err != nil {
		return nil
	}

	if l == "" {
		return nil
	}

	id, err := strconv.ParseInt(l, 10, 32)

	email, err := mailbox.ReadMessage(int(id))
	if err != nil {
		log.Panic(err)
	}

	g.Update(func(g *gocui.Gui) error {
		v, err := g.View("view")

		if err != nil {
			return err
		}

		v.Clear()

		fmt.Fprintln(v, "ID: "+l)
		fmt.Fprintln(v, "From: "+email.From)
		fmt.Fprintln(v, "Subject: "+email.Subject)
		fmt.Fprintln(v, "Date: "+email.Date)
		fmt.Fprintln(v, *email.TextBody)

		return nil
	})

	return nil
}

func receiveEmails(g *gocui.Gui) {
	defer wg.Done()

	for {
		select {
		case <-done:
			return
		case <-time.After(5000 * time.Millisecond):
			emails, err := mailbox.CheckInbox()

			if err != nil {
				log.Panic(err)
			}

			g.Update(func(g *gocui.Gui) error {
				v, err := g.View("list")

				if err != nil {
					return err
				}

				v.Clear()

				for _, email := range emails {
					fmt.Fprintln(v, email.ID)
				}

				return nil
			})
		}
	}
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	if v, err := g.SetView("address", 0, 0, maxX/5, 2); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = "Address"
		v.Wrap = true
		v.Autoscroll = true
		v.Editable = false
	}

	if v, err := g.SetView("list", 0, 3, maxX/5, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		v.Title = "List"
		v.Wrap = true
		v.Autoscroll = true
		v.Editable = false

		if _, err := g.SetCurrentView("list"); err != nil {
			return err
		}
	}

	if v, err := g.SetView("view", maxX/5+2, 0, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		//v.SelBgColor = gocui.ColorGreen
		//v.Highlight = true
		v.Title = "View"
		v.Wrap = true
		v.Autoscroll = false
		v.Editable = false
	}

	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	close(done)
	return gocui.ErrQuit
}

func nexView(g *gocui.Gui, v *gocui.View) error {
	view := "list"
	if v != nil && v.Name() == "list" {
		view = "view"
	}

	if _, err := g.SetCurrentView(view); err != nil {
		return err
	}

	return nil
}

func scrollView(v *gocui.View, dy int) error {
	if v != nil {
		v.Autoscroll = false
		ox, oy := v.Origin()
		if err := v.SetOrigin(ox, oy+dy); err != nil {
			return err
		}
	}
	return nil
}

func StringWithCharset(length int, charset string) string {
	var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func String(length int) string {
	return StringWithCharset(length, charset)
}
