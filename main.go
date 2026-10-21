package main

import (
	"database/sql"
	"flag"
	"fmt"
	"iter"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/dustin/go-humanize"
	"github.com/sahilm/fuzzy"
	_ "modernc.org/sqlite"
)

var (
	styleCursorBlue  lipgloss.Style
	styleCursorPink  lipgloss.Style
	styleCursorWhite lipgloss.Style
	styleDim         lipgloss.Style
	styleSuccess     lipgloss.Style
	styleFail        lipgloss.Style
)

func init() {
	styleCursorBlue = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	styleCursorPink = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	styleCursorWhite = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	styleDim = lipgloss.NewStyle().Faint(true)
	styleFail = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Faint(true)
}

func dbDir() string {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}

	dir = filepath.Join(dir, "stinkpot")

	return dir
}

func dbPath() string {
	return filepath.Join(dbDir(), "history.db")
}

func open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}

	dsn := "file:" + path +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(`
		create table if not exists history (
		  id      integer primary key autoincrement,
		  cmd     text not null unique,
		  cwd     text,
		  exit    integer,
		  ts      integer,
		  session text
		);

		-- walk rows newest-first and stop at the limit instead of scanning
		-- the whole table and sorting it on every invocation.
		drop index if exists history_ts_cmd;
		create index if not exists history_ts_id on history(ts desc, id desc);`); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

type candidate struct {
	ts       time.Time
	cmd      string
	exitFail bool
}

func candidates(db *sql.DB, filterByCwd bool, showFailed bool) iter.Seq[candidate] {
	const qry = `SELECT cmd, ts, cwd, exit FROM history
		ORDER BY ts DESC, id DESC
		LIMIT 10000`
	rows, err := db.Query(qry)
	if err != nil {
		panic(fmt.Errorf("%s: %w", qry, err))
	}

	currentCwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("getwd: %w", err))
	}
	return func(yield func(candidate) bool) {
		defer rows.Close()
		for rows.Next() {
			var c string
			var ts int64
			var cwd string
			var exit int64
			if err := rows.Scan(&c, &ts, &cwd, &exit); err == nil {
				if filterByCwd && cwd != currentCwd {
					continue
				}
				if !showFailed && exit != 0 {
					continue
				}
				if !yield(candidate{cmd: c, ts: time.Unix(ts, 0), exitFail: exit != 0}) {
					break
				}
			}
		}
	}
}

func add(args []string) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	exit := fs.Int("exit", 0, "exit code of the command")
	cmdFlag := fs.String("cmd", "", "command to record (alternative to positional args)")
	_ = fs.Parse(args)

	cmd := *cmdFlag
	if cmd == "" {
		cmd = strings.Join(fs.Args(), " ")
	}
	cmd = strings.TrimRight(cmd, " \t\n")
	if strings.TrimSpace(cmd) == "" {
		return
	}

	db, err := open(dbPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "stinkpot:", err)
		return
	}
	defer func() { _ = db.Close() }()

	// re-run of the same command bumps its timestamp
	cwd, _ := os.Getwd()
	if _, err := db.Exec(
		`insert into history(cmd, cwd, exit, ts, session) values(?, ?, ?, ?, ?)
		 on conflict(cmd) do update set
		   cwd     = excluded.cwd,
		   exit    = excluded.exit,
		   ts      = excluded.ts,
		   session = excluded.session`,
		cmd, cwd, *exit, time.Now().Unix(), os.Getenv("stinkpot_SESSION"),
	); err != nil {
		fmt.Fprintln(os.Stderr, "stinkpot:", err)
	}
}

func search(args []string) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	var flagNoSort *bool
	{
		s := os.Getenv("STINKPOT_NO_SORT")
		b, _ := strconv.ParseBool(s)
		flagNoSort = fs.Bool("no-sort", b, "do not sort by score (leave sorted by age)")
	}
	var flagNoCwd *bool
	{
		s := os.Getenv("STINKPOT_NO_CWD")
		b, _ := strconv.ParseBool(s)
		flagNoCwd = fs.Bool("no-cwd", b, "do not filter by cwd")
	}
	var flagNoExitFail *bool
	{
		s := os.Getenv("STINKPOT_EXIT_FAIL")
		b, _ := strconv.ParseBool(s)
		flagNoExitFail = fs.Bool("exit-fail", b, "show commands that failed when exiting")
	}

	_ = fs.Parse(args)
	initial := strings.Join(fs.Args(), " ")

	db, err := open(dbPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "stinkpot:", err)
		os.Exit(1)
	}
	defer db.Close()

	ti := textinput.New()
	ti.Prompt = "λ "
	ti.Placeholder = "search history..."
	styles := textinput.DefaultStyles(true)
	styles.Focused.Placeholder = styleDim
	styles.Blurred.Placeholder = styleDim
	ti.SetStyles(styles)
	ti.SetValue(initial)
	ti.CursorEnd()
	ti.Focus()

	m := &model{textinput: ti, db: db, sort: !*flagNoSort, cwd: !*flagNoCwd, showFailed: !*flagNoExitFail}
	m.filter()

	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	res, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stinkpot:", err)
		os.Exit(1)
	}
	if fm, ok := res.(*model); ok && fm.selected != "" {
		fmt.Println(fm.selected)
	}
}

func list(_ []string) {
	db, err := open(dbPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "stinkpot:", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`select ts, exit, cmd from history order by ts desc, id desc limit 50`)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stinkpot:", err)
		os.Exit(1)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if rows.Err() != nil {
			fmt.Fprintln(os.Stderr, "stinkpot:", err)
			os.Exit(1)
		}
		var ts int64
		var exit int
		var cmd string
		if err := rows.Scan(&ts, &exit, &cmd); err == nil {
			fmt.Printf("%s\t%d\t%s\n", time.Unix(ts, 0).Format("2006-01-02 15:04"), exit, cmd)
		}
	}
}

const maxRows = 15

type model struct {
	db         *sql.DB
	selected   string
	filtered   []candidate
	textinput  textinput.Model
	cursor     int
	sort       bool
	cwd        bool
	showFailed bool
}

// candidateStrings returns the cmd\x00ts abusing that
// fuzzy allows this and matches only till the NUL byte,
// but returns the whole string.
func candidateStrings(it iter.Seq[candidate]) iter.Seq[string] {
	return func(yield func(string) bool) {
		for c := range it {
			if !yield(fmt.Sprintf("%s\x00%x", c.cmd, c.ts.UnixMilli())) {
				break
			}
		}
	}
}

func (m *model) filter() {
	q := m.textinput.Value()

	if strings.TrimSpace(q) == "" {
		m.filtered = slices.Collect(candidates(m.db, m.cwd, m.showFailed))
	} else {
		f := fuzzy.FindFromIterNoSort
		if m.sort {
			f = fuzzy.FindFromIter
		}
		matches := f(q, candidateStrings(candidates(m.db, m.cwd, m.showFailed)))
		out := make([]candidate, len(matches))
		for i, mt := range matches {
			var c candidate
			var t string
			if c.cmd, t, _ = strings.Cut(mt.Str, "\x00"); t != "" {
				if ms, _ := strconv.ParseInt(t, 16, 64); ms != 0 {
					c.ts = time.UnixMilli(ms)
				}
			}
			out[i] = c
		}
		m.filtered = out
	}

	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}

	if m.cursor < 0 {
		m.cursor = 0
	}
}

// shortRelTime renders t as a compact "time ago" string (e.g. "3min", "2d").
func shortRelTime(t time.Time) string {
	if t.Unix() == 0 {
		return "never"
	}
	return humanize.CustomRelTime(t, time.Now(), "", "", []humanize.RelTimeMagnitude{
		{D: time.Second, Format: "now", DivBy: time.Second},
		{D: 2 * time.Second, Format: "1s", DivBy: 1},
		{D: time.Minute, Format: "%ds", DivBy: time.Second},
		{D: 2 * time.Minute, Format: "1m", DivBy: 1},
		{D: time.Hour, Format: "%dm", DivBy: time.Minute},
		{D: 2 * time.Hour, Format: "1hr", DivBy: 1},
		{D: humanize.Day, Format: "%dhrs", DivBy: time.Hour},
		{D: 2 * humanize.Day, Format: "1d", DivBy: 1},
		{D: 20 * humanize.Day, Format: "%dd", DivBy: humanize.Day},
		{D: 8 * humanize.Week, Format: "%dw", DivBy: humanize.Week},
		{D: humanize.Year, Format: "%dmo", DivBy: humanize.Month},
		{D: 18 * humanize.Month, Format: "1y", DivBy: 1},
		{D: 2 * humanize.Year, Format: "2y", DivBy: 1},
		{D: humanize.LongTime, Format: "%dy", DivBy: humanize.Year},
		{D: math.MaxInt64, Format: "a long while ago", DivBy: 1},
	})
}

func (m *model) Init() tea.Cmd { return textinput.Blink }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "ctrl+c", "esc":
			m.selected = ""
			return m, tea.Quit
		case "shift+tab":
			m.sort = !m.sort
		case "ctrl+a":
			m.showFailed = !m.showFailed
		case "tab":
			m.cwd = !m.cwd
		case "enter":
			if m.cursor >= 0 && m.cursor < len(m.filtered) {
				m.selected = m.filtered[m.cursor].cmd
			}
			return m, tea.Quit
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.textinput, cmd = m.textinput.Update(msg)
	m.filter()
	return m, cmd
}

func (m *model) View() tea.View {
	var b strings.Builder
	b.WriteString(m.textinput.View())
	b.WriteByte('\n')

	start := 0
	if m.cursor >= maxRows {
		start = m.cursor - maxRows + 1
	}
	end := min(start+maxRows, len(m.filtered))

	// width of the widest relative time in view, so commands line up.
	tsWidth := 0
	for i := start; i < end; i++ {
		if w := lipgloss.Width(shortRelTime(m.filtered[i].ts)); w > tsWidth {
			tsWidth = w
		}
	}

	for i := start; i < end; i++ {
		// Change the cursor color in the
		// pattern of the trans flag, because
		// we can be whimsical :3
		var styleCursor *lipgloss.Style
		switch i % 4 {
		case 0:
			styleCursor = &styleCursorBlue
		case 1, 3:
			styleCursor = &styleCursorPink
		default:
			styleCursor = &styleCursorWhite
		}

		c := m.filtered[i]
		line := strings.ReplaceAll(c.cmd, "\n", "  ")
		ts := styleDim.Render(fmt.Sprintf("%*s", tsWidth, shortRelTime(c.ts)))
		b.WriteString(ts)
		b.WriteByte(' ')
		b.WriteByte(' ')
		if i == m.cursor {
			b.WriteString(styleCursor.Render(line))
		} else {
			if c.exitFail {
				b.WriteString(styleFail.Render(line))
			} else {
				b.WriteString(styleDim.Render(line))
			}
		}
		b.WriteByte('\n')
	}

	filterBy := "filter by directory"
	if m.cwd {
		filterBy = "show all"
	}
	sortBy := "sort by score"
	if m.sort {
		sortBy = "sort by time"
	}
	showFailed := "show failed commands"
	if m.showFailed {
		showFailed = "hide failed"
	}
	b.WriteString(styleDim.Render(fmt.Sprintf(
		"  %d matches • ↑/↓ move • ↵ ok • esc quit • tab %s • s-tab %s • c-a %s",
		len(m.filtered), filterBy, sortBy, showFailed,
	)))
	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

func defaultAtuinPath() string {
	if p := os.Getenv("ATUIN_DB_PATH"); p != "" {
		return p
	}
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "share")
	}
	return filepath.Join(dir, "atuin", "history.db")
}

func importAtuin(args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	_ = fs.Parse(args)

	src := defaultAtuinPath()
	if fs.NArg() > 0 {
		src = fs.Arg(0)
	}

	if _, err := os.Stat(src); err != nil {
		fmt.Fprintf(os.Stderr, "stinkpot: cannot find atuin db at %s: %v\n", src, err)
		os.Exit(1)
	}

	db, err := open(dbPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "stinkpot:", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(`ATTACH DATABASE ? AS atuin`, src); err != nil {
		fmt.Fprintf(os.Stderr, "stinkpot: attach atuin db: %v\n", err)
		os.Exit(1)
	}
	defer func() { _, _ = db.Exec(`DETACH DATABASE atuin`) }()

	// Trim trailing whitespace/newlines just like add() does, convert ns -> s,
	// and only update on conflict if the imported command is newer.
	const importSQL = `
		INSERT INTO history (cmd, cwd, exit, ts, session)
		SELECT
			rtrim(command, ' ' || char(9) || char(10)),
			cwd,
			exit,
			timestamp / 1000000000,
			session
		FROM atuin.history
		WHERE deleted_at IS NULL
		  AND trim(command, ' ' || char(9) || char(10)) != ''
		ORDER BY timestamp ASC
		ON CONFLICT(cmd) DO UPDATE SET
			cwd     = excluded.cwd,
			exit    = excluded.exit,
			ts      = excluded.ts,
			session = excluded.session
		WHERE excluded.ts >= history.ts`

	res, err := db.Exec(importSQL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stinkpot: import failed: %v\n", err)
		os.Exit(1)
	}

	affected, _ := res.RowsAffected()
	var total int
	_ = db.QueryRow(`SELECT COUNT(*) FROM history`).Scan(&total)
	fmt.Fprintf(os.Stderr, "stinkpot: imported %d records from %s (%d unique commands in db)\n", affected, src, total)
}

const initScript = `# run eval "$(stinkpot init)" at startup
__stinkpot_record() {
  local exit=$?
  local cmd
  cmd=$(HISTTIMEFORMAT='' history 1 | sed '1 s/^[[:space:]]*[0-9]\{1,\}[[:space:]]*//')
  [ -n "$cmd" ] && stinkpot add --exit "$exit" -- "$cmd"
  return $exit
}
case "$PROMPT_COMMAND" in
  *__stinkpot_record*) ;;
  *) PROMPT_COMMAND="__stinkpot_record${PROMPT_COMMAND:+; $PROMPT_COMMAND}" ;;
esac

__stinkpot_search() {
  local out
  out=$(stinkpot search -- "$READLINE_LINE") || return
  if [ -n "$out" ]; then
    READLINE_LINE="$out"
    READLINE_POINT=${#READLINE_LINE}
  fi
}
bind -x '"\C-r": __stinkpot_search'
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: stinkpot <init|add|search|list|import>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		fmt.Print(initScript)
	case "add":
		add(os.Args[2:])
	case "search":
		search(os.Args[2:])
	case "list":
		list(os.Args[2:])
	case "import":
		importAtuin(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "stinkpot: unknown command %q\n", os.Args[1])
		os.Exit(1)
	}
}
