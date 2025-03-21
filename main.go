package main

import (
	"fmt"
	"image/gif"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/ncruces/zenity"
	"github.com/reujab/wallpaper"
)

var oldBackground = ""
var fps int64 = 30
var selectedGif *gif.GIF
var frames []string
var thisTempDir string
var stopAnimation chan bool

var isAnimationRunning = false;

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	
	stopAnimation = make(chan bool)
	
	tempDir := os.TempDir()
	if tempDir == "" {
		fmt.Println("Error getting temp directory")
		os.Exit(1)
	}
	
	liveDir, liveDirErr := os.MkdirTemp(tempDir, "live-wallpaper")
	if liveDirErr != nil {
		fmt.Println("Error creating temp directory:", liveDirErr)
		os.Exit(1)
	}
	
	thisTempDir = liveDir
	fmt.Println("Created temp directory:", liveDir)
	
	cacheOldBg, err := wallpaper.Get()
	if err != nil {
		fmt.Println("Error getting wallpaper:", err)
	} else {
		// copy to temp dir
		os.Create(thisTempDir + "/old-bg.jpg")
		read, err := os.ReadFile(cacheOldBg)
		if err != nil {
			fmt.Println("Error reading wallpaper:", err)
		}
		err = os.WriteFile(thisTempDir + "/old-bg.jpg", read, 0644)
		if err != nil {
			fmt.Println("Error writing wallpaper:", err)
		}
		
		oldBackground = thisTempDir + "/old-bg.jpg"
		fmt.Println("Current wallpaper:", oldBackground)

		// oldBackground = cacheOldBg // this doesn't work? need to use temp dir :(
	}
	
	go func() {
		<-sigs
		cleanup()
		os.Exit(0)
	}()
	
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTitle("wallpapah")
	selectFPS := systray.AddMenuItem("Select FPS: 30FPS", "Select the FPS for the wallpaper")
	selectGif := systray.AddMenuItem("Select GIF", "Select a GIF to set as wallpaper")
	quit := systray.AddMenuItem("Quit", "Quit the whole app")
	
	go func() {
		for {
			select {
			case <-selectFPS.ClickedCh:
				str, err := zenity.Entry("Enter FPS", zenity.Title("Enter FPS"))
				if err != nil {
					fmt.Println("Error entering FPS:", err)
					continue
				}
				if str != "" {
					fmt.Println("Entered FPS:", str)
					newFps, err := strconv.ParseInt(str, 10, 64)
					if err != nil {
						fmt.Println("Error parsing FPS:", err)
						continue
					}
					fps = newFps
					selectFPS.SetTitle("Select FPS: " + str + "FPS")
				}
			
			case <-quit.ClickedCh:
				fmt.Print("Quitting...")
				
				// cleanup();

				systray.Quit()
				return
				
			case <-selectGif.ClickedCh:
				str, err := zenity.SelectFile(zenity.FileFilter{Patterns: []string{"*.gif"}}, zenity.Title("Select a GIF file"))
				if err != nil {
					fmt.Println("Error selecting file:", err)
					continue
				}
				if str != "" {
					fmt.Println("Selected file:", str)
					file, err := os.Open(str)
					if err != nil {
						fmt.Println("Error opening file:", err)
						continue
					}
					
					realGif, err := gif.DecodeAll(file)
					file.Close()
					
					if err != nil {
						fmt.Println("Error decoding GIF:", err)
						continue
					}
					
					if selectedGif != nil {
						stopAnimation <- true
					}
					
					for _, oldFrame := range frames {
						os.Remove(oldFrame)
					}
					
					frames = []string{}
					for i, img := range realGif.Image {
						framePath := fmt.Sprintf("%s/frame-%d.gif", thisTempDir, i)
						out, err := os.Create(framePath)
						if err != nil {
							fmt.Println("Error creating frame:", err)
							continue
						}
						
						err = gif.Encode(out, img, nil)
						out.Close()
						
						if err != nil {
							fmt.Println("Error encoding frame:", err)
							continue
						}
						
						frames = append(frames, framePath)
					}
					
					fmt.Println("Folder of all frames:", thisTempDir)
					fmt.Println("Total frames:", len(frames))
					
					selectedGif = realGif
					selectGif.SetTitle("Selected GIF: " + str)
					
					go runAnimation()
				}
			}
		}
	}()
}

func runAnimation() {
	if len(frames) == 0 {
		return
	}
	
	isAnimationRunning = true
	frameIndex := 0
	
	for isAnimationRunning && selectedGif != nil {
		select {
		case <-stopAnimation:
			isAnimationRunning = false
			
		default:
			if frameIndex >= len(frames) {
				frameIndex = 0
			}
			
			fmt.Println("Setting wallpaper to frame", frameIndex)
			err := wallpaper.SetFromFile(frames[frameIndex])
			if err != nil {
				fmt.Println("Error setting wallpaper:", err)
			}
			
			frameIndex++
			time.Sleep(time.Second / time.Duration(fps))
		}
	}
}

func cleanup() {
	fmt.Println("Cleaning up...")
	
	select {
	case stopAnimation <- true:
		fmt.Println("Stopped animation")
	default:
	}

	if isAnimationRunning {
		stopAnimation <- true;
	}

	time.Sleep(100 * time.Millisecond)
	
	if oldBackground != "" {
		fmt.Println("Resetting wallpaper to:", oldBackground)
		err := wallpaper.SetFromFile(oldBackground)
		if err != nil {
			fmt.Println("Error resetting wallpaper:", err)
			time.Sleep(100 * time.Millisecond)
		}
	}
	
	if thisTempDir != "" {
		fmt.Println("Removing temp directory:", thisTempDir)
		err := os.RemoveAll(thisTempDir)
		if err != nil {
			fmt.Println("Error removing temp directory:", err)
		}
	}

}

func onExit() {
	cleanup()
}