package main

import (
	"fmt"
	"io"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// WireflyUI is a Text User Interface (TUI) for a Subscription.
// The Run method will draw the UI to the terminal in "fullscreen"
// mode. You can quit with Ctrl-C, or by typing "/quit" into the
// command prompt.
type WireflyUI struct {
	// cr        *Subscription
	ticker *Ticker

	app       *tview.Application
	peersList *tview.TextView

	msgW    io.Writer
	inputCh chan string
	doneCh  chan struct{}
}

// NewUI returns a new WireflyUI struct that controls the text UI.
// It won't actually do anything until you call Run().
func NewUI(ticker *Ticker) *WireflyUI {
	app := tview.NewApplication()

	// make a text view to contain our display messages
	msgBox := tview.NewTextView()
	msgBox.SetDynamicColors(true)
	msgBox.SetBorder(true)
	msgBox.SetTitle(fmt.Sprintf("Topic: %s", ticker.nsub.topicName))

	// text views are io.Writers, but they don't automatically refresh.
	// this sets a change handler to force the app to redraw when we get
	// new messages to display.
	msgBox.SetChangedFunc(func() {
		app.Draw()
	})

	// an input field for typing messages into
	inputCh := make(chan string, 32)
	input := tview.NewInputField().
		SetLabel("command" + " > ").
		SetFieldWidth(0).
		SetFieldBackgroundColor(tcell.ColorBlack)

	// the done func is called when the user hits enter, or tabs out of the field
	input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			// we don't want to do anything if they just tabbed away
			return
		}
		line := input.GetText()
		if len(line) == 0 {
			// ignore blank lines
			return
		}

		// bail if requested
		if line == "/quit" {
			app.Stop()
			return
		}

		// send the line onto the input chan and reset the field text
		inputCh <- line
		input.SetText("")
	})

	// make a text view to hold the list of peers in the room, updated by ui.refreshPeers()
	peersList := tview.NewTextView()
	peersList.SetBorder(true)
	peersList.SetTitle("Peers")
	peersList.SetChangedFunc(func() { app.Draw() })

	// displayPanel is a horizontal box with messages on the left and peers on the right
	// the peers list takes 20 columns, and the messages take the remaining space
	displayPanel := tview.NewFlex().
		AddItem(msgBox, 0, 1, false).
		AddItem(peersList, 20, 1, false)

	// flex is a vertical box with the displayPanel on top and the input field at the bottom.

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(displayPanel, 0, 1, false).
		AddItem(input, 1, 1, true)

	app.SetRoot(flex, true)

	return &WireflyUI{
		ticker:    ticker,
		app:       app,
		peersList: peersList,
		msgW:      msgBox,
		inputCh:   inputCh,
		doneCh:    make(chan struct{}, 1),
	}
}

// Run starts the event loop in the background, then starts
// the event loop for the text UI.
func (ui *WireflyUI) Run() error {
	go ui.handleEvents()
	defer ui.end()

	return ui.app.Run()
}

// end signals the event loop to exit gracefully
func (ui *WireflyUI) end() {
	ui.doneCh <- struct{}{}
}

// refreshPeers pulls the list of peers currently in the topic and
// displays the last 8 chars of their peer id in the Peers panel in the ui.
func (ui *WireflyUI) refreshPeers() {
	peers := ui.ticker.nsub.ListPeers() //currently connected peers from the network topic

	// clear is not threadsafe so we need to take the lock.
	ui.peersList.Lock()
	ui.peersList.Clear()
	ui.peersList.Unlock()

	for _, p := range peers {
		fmt.Fprintln(ui.peersList, shortID(p))
	}

	ui.app.Draw()
}

// displayMessage writes a DisplayMessage from the room to the message window,
// with the sender's id highlighted in green.
func (ui *WireflyUI) displayMessage(dm *DisplayMessage) {
	prompt := withColor("green", fmt.Sprintf("<%s>:", dm.SenderPeerName))
	fmt.Fprintf(ui.msgW, "%s %s\n", prompt, dm.Message)
}

// handleEvents runs an event loop that displays messages received from the topic.
// It also periodically refreshes the list of peers in the UI.
func (ui *WireflyUI) handleEvents() {
	peerRefreshTicker := time.NewTicker(time.Second)
	defer peerRefreshTicker.Stop()

	for {
		select {

		case dm := <-ui.ticker.DisplayMessages:
			// when we receive a message from the topic, print it to the message window
			ui.displayMessage(dm)

		case <-peerRefreshTicker.C:
			// refresh the list of peers in the topic periodically
			ui.refreshPeers()

		case <-ui.ticker.nsub.ctx.Done():
			return

		case <-ui.doneCh:
			return

		default:
			time.Sleep(50 * time.Millisecond)
			ui.displayMessage(nilDisplayMessage())
		}

	}
}

// withColor wraps a string with color tags for display in the messages text box.
func withColor(color, msg string) string {
	return fmt.Sprintf("[%s]%s[-]", color, msg)
}

func nilDisplayMessage() *DisplayMessage {
	dm := new(DisplayMessage)
	dm.Message = ""
	dm.SenderID = ""
	dm.SenderPeerName = ""
	return dm
}
