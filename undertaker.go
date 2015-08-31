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
	var res []docker.APIContainers

	var match_found bool

	for _, container := range containers {
		match_found = false
		// loop through pattens and check for match on ID or Names[]
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
	var res []docker.APIImages

	var match_found bool

	for _, image := range images {
		match_found = false

		// check if this image is in use
		for _,inuse := range inuseids {
			if image.ID == inuse {
				match_found = true;
				break;
			}
		}

		if match_found {
			break;
		}

		// loop over patterns and check ID and RepoTags[] for match
		for _, pattern := range patterns {
			if len(pattern.FindString(image.ID)) > 0 {
				match_found = true
				break
			}
			for _, name := range image.RepoTags {
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
			res = append(res, image)
		}
	}

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

	container_excludes, _ := loadExcludes("./container-excludes")
	still_warm_secs       := int64(0) //3600;


	var inuse_ids []string
	var containers_exited []docker.APIContainers

	containers_all, _ := client.ListContainers(docker.ListContainersOptions{All: true})
	for _, cont := range containers_all {
		//fmt.Printf("%v %v\n",cont.Names,cont.Status);
		if strings.Index(cont.Status,"Exited") == 0 {
			containers_exited = append(containers_exited,cont);
		} else {
			// we need to inspect the container for the real ID (field 
			// APIContainers.Image may hold either a name or an ID)
			tmp, _ := client.InspectContainer(cont.ID)
			inuse_ids = append(inuse_ids,tmp.Image);
			fmt.Printf("image in use: %s (%s)\n",tmp.Image, cont.Image);
		}
	}
	
	containers_exited = filterContainers(containers_exited, container_excludes)

	var containers_to_kill []*docker.Container

	for _, cont := range containers_exited {
		// TODO: first test if any exclusion filter matches the container ID or container.name

		container, _ := client.InspectContainer(cont.ID)

		if (time.Now().Unix() - container.State.FinishedAt.Unix()) < still_warm_secs {
			inuse_ids = append(inuse_ids,container.Config.Image);
		} else {
			containers_to_kill = append(containers_to_kill,container);
		}
	}

	images_all, _       := client.ListImages(docker.ListImagesOptions{All: false})
	images_excludes, _  := loadExcludes("./image-excludes")

	images_to_kill := filterImages(images_all,images_excludes,inuse_ids);


	for i, cont := range containers_to_kill {
		fmt.Printf("rm container [%2d]: %64s %v\n",i, cont.ID, cont.Name);
	}
	
	for i, img := range images_to_kill {
		fmt.Printf("rm image     [%2d]: %64s %v\n",i, img.ID, img.RepoTags);
	}
/*
		b, err := json.Marshal(img)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(string(b))
		}
*/
}

