package main

import (
	"bufio"
	//"encoding/json"
	"flag"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

func filterContainers(client *docker.Client, containers []docker.APIContainers, patterns []*regexp.Regexp,inuse_ids *[]string) []docker.APIContainers {
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
			
			fmt.Println("names=",container.Names);
			
			for _, name := range container.Names {
				// NOTE: the name starts with a slash (don't match against it)!
				if len(pattern.FindString(name[1:])) > 0 {
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
		} else {
			tmp, _ := client.InspectContainer(container.ID)
			*inuse_ids = append(*inuse_ids,tmp.Image);
			//fmt.Printf("image excluded: %s (%s)\n",tmp.Image, container.Image);
		}
	}

	return res
}

func filterImages(images []docker.APIImages, patterns []*regexp.Regexp, inuseids []string) []docker.APIImages {
	var res []docker.APIImages

	var match_found bool

	//fmt.Println("filterImages: size=",len(images))

	for _, image := range images {
		match_found = false

		// check if this image is in use
		for _,inuse := range inuseids {
			if image.ID == inuse {
				//fmt.Println("filterImages: image id ",image.ID," matches ",inuse);
				match_found = true;
				break;
			}
		}

		if match_found {
			continue;
		}

		// loop over patterns and check ID and RepoTags[] for match
		for _, pattern := range patterns {
			if len(pattern.FindString(image.ID)) > 0 {
				//fmt.Println("filterImages: pattern match against ",image.ID);
				match_found = true
				break
			}
			for _, name := range image.RepoTags {
				//fmt.Println("filterImages: pattern match against ",name);
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


// Define a type named "intslice" as a slice of ints
type stringslice []string
 
// Now, for our new type, implement the two methods of
// the flag.Value interface...
// The first method is String() string
func (i *stringslice) String() string {
    return fmt.Sprintf("%s", *i);
}
 
// The second method is Set(value string) error
func (i *stringslice) Set(value string) error {
    *i = append(*i, value)
    return nil
}
 
func main() {
	var container_excludes_file  string
	var image_excludes_file      string
	var conserving_deadline_secs int64
	var c_excludes               stringslice
	var i_excludes               stringslice


	flag.StringVar(&container_excludes_file, "fc", "/etc/undertaker/container_excludes", "exclude file for containers")
	flag.StringVar(&image_excludes_file,     "fi", "/etc/undertaker/image_excludes", "exclude file for images")
	flag.Int64Var(&conserving_deadline_secs, "wait", int64(3600), "conserving deadline in seconds")

	flag.Var(&c_excludes,"c","a single container exclude");
	flag.Var(&i_excludes,"i","a single image exclude");

 	flag.Parse()

	fmt.Println("c_excludes=",c_excludes);
	fmt.Println("i_excludes=",i_excludes);

	endpoint := "unix:///var/run/docker.sock"
	client, err := docker.NewClient(endpoint)
	if err != nil {
		log.Fatal(err)
	}

	container_excludes, _ := loadExcludes(container_excludes_file)
	for _,ex := range c_excludes {
		container_excludes = append(container_excludes,regexp.MustCompile(ex));
	}
	
	image_excludes, _ := loadExcludes(image_excludes_file)
	for _,ex := range i_excludes {
		image_excludes = append(image_excludes,regexp.MustCompile(ex));
	}

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
			//fmt.Printf("image in use: %s (%s)\n",tmp.Image, cont.Image);
		}
	}
	
	containers_exited = filterContainers(client,containers_exited, container_excludes,&inuse_ids)

	var containers_to_kill []*docker.Container

	for _, cont := range containers_exited {
		container, _ := client.InspectContainer(cont.ID)

		if (time.Now().Unix() - container.State.FinishedAt.Unix()) < conserving_deadline_secs {
			inuse_ids = append(inuse_ids,container.Config.Image);
		} else {
			containers_to_kill = append(containers_to_kill,container);
		}
	}

	images_all, _ := client.ListImages(docker.ListImagesOptions{All: false})

	images_to_kill := filterImages(images_all,image_excludes,inuse_ids);


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

