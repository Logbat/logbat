package main

import (
	"bufio"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
)

type Config struct {
	AppKey        string `yml:"app_key"`
	LogFilePath   string `yaml:"log_file_path"`
	ServerAddress string `yaml:"server_address"`
	Pattern       string `yaml:"pattern"`
}

func main() {
	ymlFile, err := os.ReadFile("config.yml")
	if err != nil {
		log.Fatal("reading yml file: $v", err)
	}
	var config Config
	err = yaml.Unmarshal(ymlFile, &config)
	if err != nil {
		log.Fatal("unmarshaling yml: %v", err)
	}

	fmt.Println("running")

	logPattern, err := regexp.Compile(config.Pattern)
	if err != nil {
		log.Fatalf("compiling log pattern: %v", err)
	}

	/*
		운영체제별 파일 감지 system call 사용
		windows의 ReadDirectoryChangesW
		linux의 inotify
		macos의 FSEvents

		비동기로 동작
		감지된 이벤트는 Events 라는 채널 변수 에 담는다
		디렉토리 감시시 하위 디렉토리도 감시 가능하다
	*/
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("fsnotify watch : %v", err)
	}
	defer watcher.Close()

	go watchFile(watcher, config.LogFilePath, logPattern)

	// 감지할 파일 등록
	err = watcher.Add(config.LogFilePath)
	if err != nil {
		log.Fatal("watcher add: %v", err)
	}
	// main 고루틴을 blocking
	// 백그라운드에서 wachFile 고루틴을 계속 실행 가능하게 한다
	<-make(chan struct{})
}

func watchFile(watcher *fsnotify.Watcher, filePath string, logPattern *regexp.Regexp) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("opening file: %v", err)
	}
	defer file.Close()

	// 파일 끝으로 이동 (함수 호출 전 기존 파일 내용 무시)
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		log.Fatalf("seeking to end of file: %v", err)
	}

	reader := bufio.NewReader(file)
	var currentLog strings.Builder

	for {
		/*
			case에 channel을 사용하는 switch 구문
			case의 channel에 값이 들어 올 때까지 select 문 blocking
		*/
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				for {
					// 다음 개행 발생 지점 까지 파일을 읽습니다
					line, err := reader.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							log.Printf("reading file: %v", err)
						}
						break
					}
					//fmt.Println("content: ", line)
					// 패턴 매칭
					if logPattern.MatchString(line) {
						if currentLog.Len() > 0 {
							fmt.Println(strings.Repeat("=", 20))
							fmt.Print(currentLog.String())
							fmt.Println(strings.Repeat("=", 20))
							currentLog.Reset()
						}
						currentLog.WriteString(line)
					} else {
						currentLog.WriteString(line)
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}
