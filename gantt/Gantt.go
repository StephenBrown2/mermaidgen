package gantt

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
)

////////// AxisFormat //////////////////////////////////////////////////////////

type axisFormat string

// Format definitions for x axis scale as described at
// https://mermaidjs.github.io/gantt.html#scale.
// The default is no axisFormat statement which results in FormatDate.
const (
	FormatDateTime24WithSeconds    axisFormat = `%Y-%m-%d %H:%M:%S`
	FormatDateTime24               axisFormat = `%Y-%m-%d %H:%M`
	FormatDateTime24Short          axisFormat = `%y%m%d %H:%M`
	FormatDate                     axisFormat = `%Y-%m-%d`
	FormatDateShort                axisFormat = `%y%m%d`
	FormatWeekdayTime24            axisFormat = `%a %H:%M`
	FormatWeekdayTime24WithSeconds axisFormat = `%a %H:%M:%S`
	FormatTime24                   axisFormat = `%H:%M`
	FormatTime24WithSeconds        axisFormat = `%H:%M:%S`
)

////////// Gantt ///////////////////////////////////////////////////////////////

// Gantt objects are the entrypoints to this package, the whole diagram is
// constructed around a Gantt object. Create an instance of Gantt via
// Gantt's constructor NewGantt, do not create instances directly.
type Gantt struct {
	sectionsMap map[string]*Section // lookup table for existing Sections
	sections    []*Section          // Section items for ordered rendering
	tasksMap    map[string]*Task    // lookup table for existing Tasks
	tasks       []*Task             // Section-less Task items
	Title       string              // Title of the Gantt diagram
	AxisFormat  axisFormat          // Optional time format for x axis
}

// NewGantt is the constructor used to create a new Gantt object.
// This object is the entrypoint for any further interactions with your diagram.
// Always use the constructor, don't create Gantt objects directly.
// Optional initializer parameters can be given in the order Title, AxisFormat.
func NewGantt(init ...interface{}) (newGantt *Gantt, err error) {
	g := &Gantt{}
	g.sectionsMap = make(map[string]*Section)
	g.tasksMap = make(map[string]*Task)
	switch l, ok := len(init), false; {
	case l > 1:
		switch v := init[1].(type) {
		case axisFormat:
			g.AxisFormat = v
		case string:
			g.AxisFormat = axisFormat(v)
		default:
			return nil, fmt.Errorf("value for AxisFormat was no axisFormat")
		}
		fallthrough
	case l > 0:
		g.Title, ok = init[0].(string)
		if !ok {
			return nil, fmt.Errorf("value for Title was no string")
		}
	}
	return g, nil
}

// String recursively renders the whole diagram to mermaid code lines.
func (g *Gantt) String() (renderedElement string) {
	renderedElement = "gantt\ndateFormat YYYY-MM-DDTHH:mm:ssZ\n"
	if g.AxisFormat != "" {
		renderedElement += fmt.Sprintln("axisFormat", g.AxisFormat)
	}
	if g.Title != "" {
		renderedElement += fmt.Sprintln("title", g.Title)
	}
	for _, t := range g.tasks {
		renderedElement += t.String()
	}
	for _, s := range g.sections {
		renderedElement += s.String()
	}
	return
}

// Structs for JSON encode
type mermaidJSON struct {
	Theme string `json:"theme"`
}

type dataJSON struct {
	Code    string      `json:"code"`
	Mermaid mermaidJSON `json:"mermaid"`
}

// LiveURL renders the Gantt and generates a view URL for
// https://mermaidjs.github.io/mermaid-live-editor from it.
func (g *Gantt) LiveURL() (url string) {
	liveURL := `https://mermaid.live/view/#pako:`
	data, _ := json.Marshal(dataJSON{
		Code: g.String(), Mermaid: mermaidJSON{Theme: "default"},
	})
	var b bytes.Buffer
	w, _ := zlib.NewWriterLevel(&b, zlib.BestCompression)
	w.Write(data)
	w.Close()
	return liveURL + base64.URLEncoding.EncodeToString(b.Bytes())
}

// ViewInBrowser uses the URL generated by Gantt's LiveURL method and opens
// that URL in the OS's default browser. It starts the browser command
// non-blocking and eventually returns any error occured.
func (g *Gantt) ViewInBrowser() (err error) {
	switch runtime.GOOS {
	case "openbsd", "linux":
		return exec.Command("xdg-open", g.LiveURL()).Start()
	case "darwin":
		return exec.Command("open", g.LiveURL()).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler",
			g.LiveURL()).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}

////////// add Items ///////////////////////////////////////////////////////////

// AddSection is used to add a new Section to this Gantt diagram. If the
// provided ID already exists, no new Section is created and an error is
// returned. The ID can later be used to look up the created Section using
// Gantt's GetSection method.
func (g *Gantt) AddSection(id string) (newSection *Section, err error) {
	newSection, err = sectionNew(id, g)
	if err != nil {
		return
	}
	g.sectionsMap[id] = newSection
	g.sections = append(g.sections, newSection)
	return
}

// AddTask is used to add a new Task to this Gantt diagram. If the provided ID
// already exists or is invalid, no new Task is created and an error is
// returned. The ID can later be used to look up the created Task using Gantt's
// GetTask method. Optional initializer parameters can be given in the order
// Title, Duration, Start, Critical, Active, Done. Duration and Start are set
// via Task's SetDuration and SetStart respectively.
func (g *Gantt) AddTask(id string, init ...interface{}) (newTask *Task, err error) {
	newTask, err = taskNew(id, g, nil, init)
	if err != nil {
		return
	}
	g.tasksMap[id] = newTask
	g.tasks = append(g.tasks, newTask)
	return
}

////////// get Items ///////////////////////////////////////////////////////////

// GetSection looks up a previously defined Section by its ID.
// If this ID doesn't exist, nil is returned.
// Use Gantt's AddSection to create new Sections.
func (g *Gantt) GetSection(id string) (existingSection *Section) {
	// if not found -> nil
	return g.sectionsMap[id]
}

// GetTask looks up a previously defined Task by its ID.
// If this ID doesn't exist, nil is returned.
// Use Gantt's or Section's AddTask to create new Tasks.
func (g *Gantt) GetTask(id string) (existingTask *Task) {
	// if not found -> nil
	return g.tasksMap[id]
}

////////// list Items //////////////////////////////////////////////////////////

// ListSections returns a slice of all Sections previously added to this
// Gantt diagram in the order they were defined.
func (g *Gantt) ListSections() (allSections []*Section) {
	allSections = make([]*Section, len(g.sections))
	copy(allSections, g.sections)
	return
}

// ListLocalTasks returns a slice of all Tasks previously added to this
// Gantt diagram directly (not to Sections) in the order they were defined.
func (g *Gantt) ListLocalTasks() (localTasks []*Task) {
	localTasks = make([]*Task, len(g.tasks))
	copy(localTasks, g.tasks)
	return
}

// ListTasks returns a slice of all Tasks previously added to this
// Gantt diagram and all of its Sections in alphabetic order by ID.
func (g *Gantt) ListTasks() (allTasks []*Task) {
	allTasks = make([]*Task, 0, len(g.tasksMap))
	for _, v := range g.tasksMap {
		allTasks = append(allTasks, v)
	}
	// sort the slice of structs by ID field to provide constant order
	sort.Slice(allTasks, func(i, j int) bool {
		return allTasks[i].id < allTasks[j].id
	})
	return
}
