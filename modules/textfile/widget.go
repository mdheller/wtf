package textfile

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/radovskyb/watcher"
	"github.com/rivo/tview"
	"github.com/wtfutil/wtf/utils"
	"github.com/wtfutil/wtf/wtf"
)

type Widget struct {
	wtf.KeyboardWidget
	wtf.MultiSourceWidget
	wtf.TextWidget

	settings *Settings
}

// NewWidget creates a new instance of a widget
func NewWidget(app *tview.Application, pages *tview.Pages, settings *Settings) *Widget {
	widget := Widget{
		KeyboardWidget:    wtf.NewKeyboardWidget(app, pages, settings.common),
		MultiSourceWidget: wtf.NewMultiSourceWidget(settings.common, "filePath", "filePaths"),
		TextWidget:        wtf.NewTextWidget(app, settings.common, true),

		settings: settings,
	}

	// Don't use a timer for this widget, watch for filesystem changes instead
	widget.settings.common.RefreshInterval = 0

	widget.initializeKeyboardControls()
	widget.View.SetInputCapture(widget.InputCapture)

	widget.SetDisplayFunction(widget.display)
	widget.View.SetWordWrap(true)
	widget.View.SetWrap(true)

	widget.KeyboardWidget.SetView(widget.View)

	go widget.watchForFileChanges()

	return &widget
}

/* -------------------- Exported Functions -------------------- */

// Refresh is only called once on start-up. Its job is to display the
// text files that first time. After that, the watcher takes over
func (widget *Widget) Refresh() {
	widget.display()
}

func (widget *Widget) HelpText() string {
	return widget.KeyboardWidget.HelpText()
}

/* -------------------- Unexported Functions -------------------- */

func (widget *Widget) display() {
	title := fmt.Sprintf("[green]%s[white]", widget.CurrentSource())

	_, _, width, _ := widget.View.GetRect()
	text := widget.settings.common.SigilStr(len(widget.Sources), widget.Idx, width) + "\n"

	if widget.settings.format {
		text += widget.formattedText()
	} else {
		text += widget.plainText()
	}

	widget.Redraw(title, text, true)
}

func (widget *Widget) fileName() string {
	return filepath.Base(widget.CurrentSource())
}

func (widget *Widget) formattedText() string {
	filePath, _ := utils.ExpandHomeDir(widget.CurrentSource())

	file, err := os.Open(filePath)
	if err != nil {
		return err.Error()
	}

	lexer := lexers.Match(filePath)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	style := styles.Get(widget.settings.formatStyle)
	if style == nil {
		style = styles.Fallback
	}
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	contents, _ := ioutil.ReadAll(file)
	iterator, _ := lexer.Tokenise(nil, string(contents))

	var buf bytes.Buffer
	formatter.Format(&buf, style, iterator)

	return tview.TranslateANSI(buf.String())
}

func (widget *Widget) plainText() string {
	filePath, _ := utils.ExpandHomeDir(widget.CurrentSource())

	fmt.Println(filePath)

	text, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err.Error()
	}
	return string(text)
}

func (widget *Widget) watchForFileChanges() {
	watch := watcher.New()
	watch.FilterOps(watcher.Write)

	go func() {
		for {
			select {
			case <-watch.Event:
				widget.display()
			case err := <-watch.Error:
				log.Fatalln(err)
			case <-watch.Closed:
				return
			}
		}
	}()

	// Watch each textfile for changes
	for _, source := range widget.Sources {
		fullPath, err := utils.ExpandHomeDir(source)
		if err == nil {
			if err := watch.Add(fullPath); err != nil {
				log.Fatalln(err)
			}
		}
	}

	// Start the watching process - it'll check for changes every 100ms.
	if err := watch.Start(time.Millisecond * 100); err != nil {
		log.Fatalln(err)
	}
}
