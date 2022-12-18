package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"wstest/client"
	"wstest/common"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gorilla/websocket"
)

var controlMessage = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00FF"))

type model struct {
	c         *client.Client
	textInput textinput.Model
	quit      bool
	err       error
}

func newModel(c *client.Client) *model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Focus()
	return &model{
		c:         c,
		textInput: ti,
		quit:      false,
		err:       nil,
	}
}

func (m *model) readMessage() tea.Msg {
	message, err := m.c.Receive()
	if err != nil {
		return err
	}
	return message
}

func (m *model) writeMessage(message string) tea.Cmd {
	return func() tea.Msg {
		err := m.c.Send([]byte(message))
		return err
	}
}
func (m *model) Init() tea.Cmd {
	return m.readMessage
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			message := m.textInput.Value()
			m.textInput.Reset()
			if len(message) == 0 {
				return m, nil
			}
			return m, m.writeMessage(message)

		case tea.KeyCtrlC:
			m.quit = true
			m.c.Close()
			return m, tea.Quit
		}
	case *common.Message:
		switch msg.Type {
		case common.ConnectedMessage:
			cmd = tea.Println(controlMessage.Render(fmt.Sprintf("%s connected!", msg.Name)))
		case common.DisconnectedMessage:
			cmd = tea.Println(controlMessage.Render(fmt.Sprintf("%s disconnected!", msg.Name)))
		case common.ListMessage:
			count := strings.Count(msg.Message, ",") + 1
			fmt := "%d Users Online %s"
			if count == 1 {
				fmt = "%d User Online %s"
			}
			cmd = tea.Printf(fmt, count, msg.Message)
		case common.TextMessage:
			cmd = tea.Printf("%s: %s", msg.Name, msg.Message)
		}
		return m, tea.Batch(cmd, m.readMessage)

	case error:
		m.quit = true
		return m, tea.Quit
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *model) View() string {
	if m.quit {
		return ""
	}

	return "\n" + m.textInput.View()
}

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) < 4 {
		fmt.Print("Usage: client [NAME] [ADDRESS] [SERVER_CERT]\n")
		os.Exit(1)
	}
	pem, err := os.ReadFile(os.Args[3])
	checkErr(err)

	dialer := websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			opts := x509.VerifyOptions{
				Roots: x509.NewCertPool(),
			}
			opts.Roots.AppendCertsFromPEM(pem)
			_, err := cs.PeerCertificates[0].Verify(opts)
			return err
		},
	}
	conn, _, err := dialer.Dial("wss://"+os.Args[2], nil)
	checkErr(err)

	c, err := client.Upgrade(conn, os.Args[1])
	checkErr(err)

	p := tea.NewProgram(newModel(c))
	_, err = p.Run()
	checkErr(err)

}
