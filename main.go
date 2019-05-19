package main

import (
	"fmt"
	"image/color"
	"image/png"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DeleteAfterAnaylsis = true
)

var (
	factorioDir = `/factorio/bin/x64/`
	// paths can be relative to factoriodir
	previewDir = `./../../../factoriotest`
	minSeed    = 78330000
	maxSeed    = 78330500
	maxProcs   = 8

	copperColor = color.NRGBA{0xCB, 0x61, 0x35, 0xff}
	ironColor   = color.NRGBA{0x68, 0x84, 0x92, 0xff}
)

var outputDir string

func init() {
	outputDir = previewDir
	if strings.Index(outputDir, ".") == 0 {
		outputDir = factorioDir + previewDir[1:]
	}

}

func main() {
	genSettings, err := ioutil.ReadFile("./settings/gen-settings.json")
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(fmt.Sprintf("%s/%s", outputDir, "gen-settings.json"), genSettings, os.ModePerm)
	if err != nil {
		panic(err)
	}

	results := make([]mapProperties, 0, maxSeed-minSeed+1)
	jobCh, resultCh := make(chan int), make(chan int, 100)

	for i := 0; i < maxProcs; i++ {
		go factorioWorker(i, jobCh, resultCh)
	}

	go func() {
		for seed := minSeed; seed <= maxSeed; seed++ {
			jobCh <- seed
			if seed%100 == 0 {
				fmt.Println("processed seed", seed)
			}
		}
		close(jobCh)
	}()

	var wg sync.WaitGroup
	var m sync.Mutex

	start := time.Now()

	for seed := minSeed; seed <= maxSeed; seed++ {
		r := <-resultCh

		wg.Add(1)
		go func() {
			defer wg.Done()

			p := analyseMap(r)

			m.Lock()
			results = append(results, p)
			m.Unlock()
		}()
	}

	wg.Wait()
	fmt.Println("that took:", time.Since(start))

	copperWeight := 1.0

	sort.Slice(results, func(i, j int) bool {
		less := int(copperWeight*float64(results[i].copper))+results[i].iron >
			int(copperWeight*float64(results[j].copper))+results[j].iron

		return less
	})

	res := ""
	for _, r := range results {
		res += r.String() + "\n"
	}

	ioutil.WriteFile(fmt.Sprintf("%s/result%d-%d.txt", outputDir, minSeed, maxSeed), []byte(res), os.ModePerm)
}

func factorioWorker(id int, seeds <-chan int, results chan<- int) {
	err := os.Mkdir(fmt.Sprintf(`%s/worker%d`, outputDir, id), os.ModeDir)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	// create config file
	bs, err := ioutil.ReadFile("./config.ini.template")
	if err != nil {
		panic(err)
	}

	updated := strings.Replace(string(bs), "NNNNN", strconv.Itoa(id), 1)
	config := fmt.Sprintf(`%s/worker%d/config.ini`, outputDir, id)

	err = ioutil.WriteFile(config, []byte(updated), os.ModePerm)
	if err != nil {
		panic(err)
	}

	for seed := range seeds {
		filename := fmt.Sprintf(`%s/seed%d.png`, previewDir, seed)
		// TODO: most these settings are pulled out of thin air
		cmd := exec.Command("./factorio",
			"--config", fmt.Sprintf("%s/worker%d/config.ini", previewDir, id),
			"--mod-directory", "./../../mods",
			"--executable-path", "./",
			"--generate-map-preview", filename,
			"--slope-shading", "0",
			"--map-preview-size", "128",
			"--no-log-rotation",
			"--map-preview-scale", "4",
			"--threads", "1",
			"--map-gen-settings", fmt.Sprintf("%s/gen-settings.json", previewDir),
			"--map-gen-seed", strconv.Itoa(seed))
		cmd.Dir = factorioDir

		//cmd.Stdout = ioutil.Discard

		if err := cmd.Run(); err != nil {
			// what happened???
			fmt.Println("err:", err, "seed:", seed)
		}

		results <- seed
	}
}

type mapProperties struct {
	seed   int
	iron   int
	copper int
}

func (m mapProperties) String() string {
	return fmt.Sprintf("seed %d:\n\tiron: %d copper: %d", m.seed, m.iron, m.copper)
}

func analyseMap(seed int) mapProperties {
	filename := fmt.Sprintf(`%s/seed%d.png`, outputDir, seed)

	f, err := os.Open(filename)
	if err != nil {
		fmt.Println("err opening file", filename, ":", err)
		return mapProperties{}
	}

	// awkward, but we must close the file, especially if we are deleting it
	if DeleteAfterAnaylsis {
		defer func() {
			f.Close()

			err := os.Remove(filename)
			if err != nil {
				fmt.Println("err deleting file:", err)
				// what happened??
				return
			}
		}()
	} else {
		defer f.Close()
	}

	src, err := png.Decode(f)
	if err != nil {
		// what happened?
		fmt.Println("err decoding file:", err)
		return mapProperties{}
	}

	bounds := src.Bounds()
	var copper, iron int

	for x := 0; x < bounds.Max.X; x++ {
		for y := 0; y < bounds.Max.Y; y++ {
			c := src.At(x, y).(color.NRGBA)
			if c == ironColor {
				iron++
				continue
			}

			if c == copperColor {
				copper++
				continue
			}
		}
	}

	return mapProperties{seed: seed, copper: int(copper), iron: int(iron)}
}
