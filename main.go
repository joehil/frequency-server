package main

import (
    "html/template"
    "net/http"
    "io/ioutil"
    "fmt"
    "os"
    "time"
    "bufio"
    "strings"
    "strconv"
    "github.com/tarm/serial"
)

type FFiles struct {
    Frfile string
}

type FormsData struct {
    Title string
    Frmethod string
    Frfiles []FFiles
    Frfile string
    Stage string
    Frequency string
    Amplitude string
    Waveform string
    TimeToGo string
}

type Answer struct {
    Frmethod string
    Frfile string
    Stage string
    Until string
}

func listDir(direc string) (trfiles []FFiles) {
    files, err := ioutil.ReadDir(direc)
    if err == nil {
	    for _, file := range files {
                tfile := FFiles{
			Frfile: file.Name(),
		}
                trfiles = append(trfiles,tfile)
    	    }
   }
   return
}

var timeToGo string = "0"
var frequency string = "0"
var amplitude string = "0"
var waveform string = "0"
var curFile string = ""
var hasEnded bool = false

var lostart int = 0
var loend int = 0
var locnt int = 0
var lotime string = ""
var isLoop bool = false
var stopFlag bool = false

var isRunning bool = false

func main() {
    var data FormsData

    data.Frmethod = "Audio"

    fmt.Println("Frequency server started")

    chome := os.Getenv("HOME")

    fmt.Println("HOME: "+chome)

    cusb := os.Getenv("USBPORT")

    fmt.Println("USBPORT: "+cusb)

    cport := os.Getenv("WEBPORT")

    fmt.Println("WEBPORT: "+cport)

    tmpl := template.Must(template.ParseFiles(chome+"/forms.html"))

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

        answer := Answer{
            Frmethod: r.FormValue("frmethod"),
            Frfile: r.FormValue("frfile"),
            Stage: r.FormValue("stage"),
	    Until: r.FormValue("loopuntil"),
        }

	if answer.Frfile != "" {
		curFile = answer.Frfile
	}

//	fmt.Println(answer)

        if answer.Frmethod == "" {
		answer.Frmethod = "Audio"
        }

        if answer.Stage == "" {
                answer.Stage = "Initial"
        }

        if answer.Stage == "Stop" {
		stopFlag = true
                answer.Stage = "Run"
		fmt.Println("Abort initiated")
        }

        data = FormsData{
		Title: "Kein Titel",
		Frmethod: answer.Frmethod, 
            	Frfiles: listDir(chome+"/data/"+answer.Frmethod),
		Frfile: answer.Frfile, 
		Stage: answer.Stage,
        }

//	fmt.Println(data)

	if answer.Stage == "Success" {
		fmt.Println("File "+answer.Frfile+" chosen")
		switch answer.Frmethod {
        		case "Audio":
                        fmt.Println("Audio: "+answer.Frfile)
//        		procAudio("data/"+answer.Frfile)
        		case "FY2300":
			fmt.Println("FY2300: "+answer.Frfile, answer.Until)
        		go procFy2300(chome+"/data/FY2300/"+answer.Frfile,answer.Until,cusb)
		        default:
        		fmt.Println("The command is wrong!")
			data.Stage = "Run"
    		}
	}

	if  data.Stage == "Run" || isRunning {
		data.TimeToGo = timeToGo
                data.Frequency = frequency
                data.Amplitude = amplitude
                data.Waveform = waveform
		data.Frfile = curFile
		data.Stage = "Run"
		if hasEnded {
			data.Stage = "Ended"
			hasEnded = false
			stopFlag = false
		}
	}

        tmpl.Execute(w, data)
    })

    http.ListenAndServe(":"+cport, nil)
}

func procFy2300(path string, loopuntil string, cusb string){
    isRunning = true
    loop := strings.Replace(loopuntil, ":", ".",-1)
    lines,err := readLines(path)

    if err != nil {
	fmt.Println(err)
    }

    c := &serial.Config{Name: "/dev/serial/by-id/"+cusb, Baud: 115200}
    s, err := serial.OpenPort(c)
    if err != nil {
    	fmt.Println(err)
    } 

    for ind := 0; ind < len(lines); ind++ {
	cmd := lines[ind]
	cser, cint, p := parseFy2300(cmd)
	switch p[0] {
		case "fr":
		frequency = p[1]
		case "am":
		amplitude = p[1]
		case "wv":
		waveform = p[1]
		default:
	}
	if cint != "" {
		pt := strings.Split(cint, ":")
		if pt[0] == "do" {
			timeToGo = cint
			limit, err := strconv.Atoi(pt[1])
   			if err != nil {
        			fmt.Println(err)
	    		} 
			for n:=0; n<limit; n++ {
	                        if isLoop {
        	                        now := time.Now()
                	                tim := fmt.Sprintf("%02d.%02d",now.Hour(),now.Minute())
                        	        if n % 10 == 0 {
						fmt.Println(tim+" - "+lotime)
					}
                                	if tim == lotime {
                                        	ind = loend
						fmt.Println("Loop finished")
						n = limit + 1
                                	}
                        	}
				timeToGo = fmt.Sprintf("%d",limit-n)
                        	time.Sleep(1 * time.Second)
				if stopFlag {
					n = limit + 1
					ind = len(lines) 
					cser = "WMN0"
					fmt.Println("Process aborted")
				}
                	} 
		}
                if pt[0] == "lo" {
			lostart = ind
			fmt.Println("Loop initiated")
		}
                if pt[0] == "un" {
                        locnt++
			limit, _ := strconv.Atoi(pt[1])
			fmt.Println(limit,locnt)
			if limit > locnt {
				ind = lostart
			} else {
				fmt.Println("Loop finished")
			}
                }
                if pt[0] == "ti" {
			loend = ind 
			isLoop = true
			lotime = strings.Replace(pt[1], "<UNTIL>", loop, -1)
                        ind = lostart
                }
	}
	if cser != "" {
		fmt.Println(cser)
        	_, err := s.Write([]byte(cser+"\n"))
        	if err != nil {
                	fmt.Println(err)
        	}
		time.Sleep(1 * time.Second)
	}
    }
    hasEnded = true
    isRunning = false
    lostart = 0
    loend = 0
    lotime = ""
    locnt = 0
    isLoop = false
}

func readLines(path string) ([]string, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var lines []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }
    return lines, scanner.Err()
}

func parseFy2300(cmd string) (string, string, []string) {
    var cser string = ""
    var cint string = ""

    parts := strings.Split(cmd, " ")

    switch parts[0] {
        case "do":
        cint = "do:"+parts[1]
        case "lo":
        cint = "lo"
        case "un":
        cint = "un:"+parts[1]
        case "ti":
        cint = "ti:"+parts[1]
	case "fr":
        cser = "WMF"+parts[1]
        case "am":
        cser = "WMA"+parts[1]
        case "wv":
        cser = "WMW"+parts[1]
        case "on":
        cser = "WMN1"
        case "of":
        cser = "WMN0"
	case "##":
	break
        default:
        fmt.Println("The command is wrong!")
    }

    return cser, cint, parts
}

