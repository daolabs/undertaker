package main

import (
	"bufio"
	//"encoding/json"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

func filterContainers(containers []docker.APIContainers, patterns []*regexp.Regexp) []docker.APIContainers {
	// apply filters against container.ID and container.Names[]
	var res []docker.APIContainers

	var match_found bool

	for _, container := range containers {
		match_found = false
		for _, pattern := range patterns {
			if len(pattern.FindString(container.ID)) > 0 {
				match_found = true
				break
			}
			for _, name := range container.Names {
				if len(pattern.FindString(name)) > 0 {
					match_found = true
					break
				}
			}
			if match_found {
				break
			}
		}
		if !match_found {
			res = append(res, container)
		}
	}

	return res
}

func filterImages(images []docker.APIImages, patterns []*regexp.Regexp, inuseids []string) []docker.APIImages {
	// apply filters against image.ID and image.RepoTags
	var res []docker.APIImages

	return res
}

// loads exclude file removing comments and empty lines
func loadExcludes(filename string) ([]*regexp.Regexp, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	res := make([]*regexp.Regexp, 0)
	var pattern *regexp.Regexp

	re_comment_prefix := regexp.MustCompile("^#.*")
	re_comment_postfix := regexp.MustCompile("#.*$")

	scanner := bufio.NewScanner(file)
	text := ""
	for scanner.Scan() {
		// remove leading and trailing spaces
		text = strings.TrimSpace(scanner.Text())

		// remove lines starting with comment char (lines starting with #)
		if text = re_comment_prefix.ReplaceAllString(text, ""); len(text) == 0 {
			continue
		}
		// remove postfix comments (comments at the end of a line)
		if text = re_comment_postfix.ReplaceAllString(text, ""); len(text) == 0 {
			continue
		}
		// trim spaces again
		if text = strings.TrimSpace(text); len(text) == 0 {
			continue
		}

		if pattern, err = regexp.Compile(text); err != nil {
			log.Println("invalid exclusion pattern: [", text, " ] - ", err)
			continue
		}

		res = append(res, pattern)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func main() {
	endpoint := "unix:///var/run/docker.sock"
	client, err := docker.NewClient(endpoint)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(client)

	container_excludes, _ := loadExcludes("./container-excludes")
	fmt.Println(container_excludes)

	filters := make(map[string][]string)
	filters["status"] = []string{"exited"}

	container_exited, _ := client.ListContainers(docker.ListContainersOptions{All: true, Filters: filters})

	container_exited = filterContainers(container_exited, container_excludes)

	for i, cont := range container_exited {
		// TODO: first test if any exclusion filter matches the container ID or container.name

		container, _ := client.InspectContainer(cont.ID)
		fmt.Printf("ALL: container[%d] %v %v %v\n", i, container.ID, container.Name, container.State.FinishedAt)
		fmt.Println(time.Now().Unix() - container.State.FinishedAt.Unix())

		// check if time diff is long enough -> if not: at container.Config.Image to the INUSE list
		// ELSE: add container structure to the take-under list
		/*
			b, err := json.Marshal(container)
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println(string(b))
			}
		*/
	}

	//container_running,_ := client.ListContainers(docker.ListContainersOptions{All:false})

	//images_all, _ := client.ListImages(docker.ListImagesOptions{All: false})

	//for i, container := range container_running {
	//	fmt.Printf("RUN: container[%d] %v\n",i,container);
	//}

	//for i, img := range images_all {
	//	fmt.Printf("image [%d] %v\n",i,img);
	//}

	//image_excludes := loadExcludes(IMAGE_EXCLUDES);

	//imgs, _ := client.ListImages(docker.ListImagesOptions{All: false})
	//for _, img := range imgs {

	// TODO:
	// (1) remove in use-images
	// (2) filter out exlusions (either img.ID or img.RepoTags[]
	/*
		b, err := json.Marshal(img)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(string(b))
		}
	*/
	//}

}
