package main

import (
	"crypto/md5"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL  = "http://192.168.70.237:2080"
	username = "osi"
	password = ""
	maxPages = 50
)

type Register struct {
	PointNum    int
	Description string
	Value       string
	Unit        string
	PointType   string
}

var debug bool

func main() {
	flag.BoolVar(&debug, "debug", false, "Enable debug output")
	flag.Parse()

	// Create cookie jar with proper options
	jar, err := cookiejar.New(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create cookie jar: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		// Don't follow redirects automatically so we can see what's happening
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if debug {
				fmt.Fprintf(os.Stderr, "  Redirect: %s -> %s\n", via[len(via)-1].URL, req.URL)
			}
			return nil
		},
	}

	// Login
	fmt.Fprintf(os.Stderr, "Logging in...\n")
	pk, err := getPK(client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to get nonce: %v\n", err)
		os.Exit(1)
	}
	hash := md5Hash(password + pk)
	oui, err := login(client, username, hash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Login failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Logged in (oui=%s)\n", oui)

	// Scrape all pages
	fmt.Fprintf(os.Stderr, "Scraping pages 1-39...\n")
	pageNums, _ := getPageList(client, oui)
	var allRegisters []Register

	for _, page := range pageNums {
		pageURL := fmt.Sprintf("%s/indexPt.html?oui=%s&pg=%d", baseURL, oui, page)

		registers, _, err := fetchPointPage(client, pageURL)
		if err != nil || len(registers) == 0 {
			continue
		}
		allRegisters = append(allRegisters, registers...)
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Fprintf(os.Stderr, "Found %d registers\n", len(allRegisters))
	outputGoCode(allRegisters)
}

func getPK(client *http.Client) (string, error) {
	resp, err := client.Get(baseURL + "/pk.js")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Parse: pk = '46D69F9';
	re := regexp.MustCompile(`pk\s*=\s*'([^']+)'`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return "", fmt.Errorf("could not parse pk from: %s", string(body))
	}

	return matches[1], nil
}

func md5Hash(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}

func login(client *http.Client, user, hash string) (string, error) {
	postBody := fmt.Sprintf("Pnamef=%s&Pwordf=%s", user, hash)
	req, err := http.NewRequest("POST", baseURL+"/postPW", strings.NewReader(postBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) // consume body

	// Verify login by checking a protected page with oui=2
	testResp, err := client.Get(baseURL + "/pointVars.js?oui=2&pg=1")
	if err != nil {
		return "", fmt.Errorf("failed to verify: %w", err)
	}
	testBody, _ := io.ReadAll(testResp.Body)
	testResp.Body.Close()

	levelRe := regexp.MustCompile(`levelStr\s*=\s*'(-?\d+)'`)
	if m := levelRe.FindStringSubmatch(string(testBody)); len(m) >= 2 {
		level, _ := strconv.Atoi(m[1])
		if level >= 0 {
			return "2", nil
		}
		return "", fmt.Errorf("not authenticated (levelStr=%d)", level)
	}
	return "", fmt.Errorf("could not verify authentication")
}

func getPageList(client *http.Client, oui string) ([]int, error) {
	// Just try all pages from 1 to 39
	// Pages aren't contiguous - different panels have different features
	var pageNums []int
	for i := 1; i <= 39; i++ {
		pageNums = append(pageNums, i)
	}
	return pageNums, nil
}

func fetchPointPage(client *http.Client, pageURL string) ([]Register, bool, error) {
	resp, err := client.Get(pageURL)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}

	html := string(body)

	if debug {
		fmt.Fprintf(os.Stderr, "  Response length: %d bytes\n", len(html))
	}

	// Parse point data directly from HTML
	// Format: <td id="PtNum1" class="PtNum">1</td>
	//         <td id="PtDesc1" class="PtDesc">RT AlarmStatus</td>
	//         <td id="PtType1" class="PtType">L</td>
	//         <td id="PtUnit1" class="PtUnit"></td>

	registers := parsePointsFromHTML(html)

	// Also get current values from pointVars.js
	varsURL := strings.Replace(pageURL, "indexPt.html", "pointVars.js", 1)
	varsResp, err := client.Get(varsURL)
	if err == nil {
		defer varsResp.Body.Close()
		varsBody, _ := io.ReadAll(varsResp.Body)
		varsJS := string(varsBody)

		ptVals := parseJSArray(varsJS, "ptVals")
		ptUnits := parseJSArray(varsJS, "ptUnits")

		// Merge values into registers
		for i := range registers {
			if i < len(ptVals) && ptVals[i] != "" {
				registers[i].Value = ptVals[i]
			}
			if i < len(ptUnits) && ptUnits[i] != "" && registers[i].Unit == "" {
				registers[i].Unit = ptUnits[i]
			}
		}
	}

	hasMore := strings.Contains(html, "Next Page")
	return registers, hasMore, nil
}

func parsePointsFromHTML(html string) []Register {
	var registers []Register

	// Extract PtNum and PtDesc pairs from HTML
	// <td id="PtNum1" class="PtNum">1</td>
	// <td id="PtDesc1" class="PtDesc">RT AlarmStatus</td>

	numRe := regexp.MustCompile(`<td[^>]*id="PtNum(\d+)"[^>]*>(\d*)</td>`)
	descRe := regexp.MustCompile(`<td[^>]*id="PtDesc(\d+)"[^>]*>([^<]*)</td>`)
	typeRe := regexp.MustCompile(`<td[^>]*id="PtType(\d+)"[^>]*>([^<]*)</td>`)
	unitRe := regexp.MustCompile(`<td[^>]*id="PtUnit(\d+)"[^>]*>([^<]*)</td>`)

	nums := numRe.FindAllStringSubmatch(html, -1)
	descs := descRe.FindAllStringSubmatch(html, -1)
	types := typeRe.FindAllStringSubmatch(html, -1)
	units := unitRe.FindAllStringSubmatch(html, -1)

	if debug {
		fmt.Fprintf(os.Stderr, "  HTML parsing: %d nums, %d descs, %d types, %d units\n",
			len(nums), len(descs), len(types), len(units))
	}

	// Build a map of index -> data
	descMap := make(map[string]string)
	typeMap := make(map[string]string)
	unitMap := make(map[string]string)

	for _, m := range descs {
		descMap[m[1]] = strings.TrimSpace(m[2])
	}
	for _, m := range types {
		typeMap[m[1]] = strings.TrimSpace(m[2])
	}
	for _, m := range units {
		unitMap[m[1]] = strings.TrimSpace(m[2])
	}

	for _, m := range nums {
		idx := m[1]
		ptNumStr := strings.TrimSpace(m[2])
		if ptNumStr == "" {
			continue
		}

		ptNum, err := strconv.Atoi(ptNumStr)
		if err != nil {
			continue
		}

		desc := descMap[idx]
		if desc == "" {
			continue // Skip empty entries
		}

		reg := Register{
			PointNum:    ptNum,
			Description: desc,
			PointType:   typeMap[idx],
			Unit:        unitMap[idx],
		}

		registers = append(registers, reg)
	}

	return registers
}

func parseJSArray(js, name string) []string {
	re := regexp.MustCompile(name + `\s*=\s*\[([\s\S]*?)\];`)
	matches := re.FindStringSubmatch(js)
	if len(matches) < 2 {
		return nil
	}

	// Extract quoted strings
	itemRe := regexp.MustCompile(`'([^']*)'`)
	items := itemRe.FindAllStringSubmatch(matches[1], -1)

	var result []string
	for _, item := range items {
		if len(item) >= 2 {
			result = append(result, item[1])
		}
	}
	return result
}

func outputGoCode(registers []Register) {
	fmt.Println("package main")
	fmt.Println("// Orenco registers scraped from web interface")
	fmt.Println("// Formula: Modbus base = (PointNum * 2) + 999 for float32")
	fmt.Println("//          Modbus base = 40000 + PointNum for int/digital")
	fmt.Println("")
	fmt.Println("var orencoRegisters = []struct {")
	fmt.Println("\tpointNum int")
	fmt.Println("\tname     string")
	fmt.Println("\tbaseF32  uint16  // float32: pointNum*2 + 999")
	fmt.Println("\tbaseInt  uint16  // int/digital: 40000 + pointNum")
	fmt.Println("\tptype    string  // A=analog, D=digital, L=label, M=?, T=time")
	fmt.Println("\tunit     string")
	fmt.Println("}{")

	for _, r := range registers {
		baseF32 := r.PointNum*2 + 999
		baseInt := 40000 + r.PointNum
		safeName := sanitizeName(r.Description)
		fmt.Printf("\t{%d, %q, %d, %d, %q, %q}, // %s\n",
			r.PointNum, safeName, baseF32, baseInt, r.PointType, r.Unit, r.Description)
	}
	fmt.Println("}")
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "#", "Num")
	s = strings.ReplaceAll(s, "%", "Pct")
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
