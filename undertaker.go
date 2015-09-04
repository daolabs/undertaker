package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"log"
	"os"
	"regexp"
	"strings"
//	"time"
)

// MultiStringOption is a commandline string option which can be specified 
// multiple times and will be parsed by flag.Var() / flag.Parse() into an
// array.
type MultiStringOption []string

// implementation of flag.Value interface (part 1)
func (i *MultiStringOption) String() string {
    return fmt.Sprintf("%s", *i);
}

// implementation of flag.Value interface (part 2)
func (i *MultiStringOption) Set(value string) error {
	*i = append(*i, value)
	return nil
}

type RegexpHolder struct {
	re *regexp.Regexp
	RegExp string
}


type UndertakerData struct {
	ContainerExc     *[]RegexpHolder
	ContainerInc     *[]RegexpHolder
	ImageExc         *[]RegexpHolder
	ImageInc         *[]RegexpHolder
	client            *docker.Client
	ContainersAll     []docker.APIContainers
	ContainersExited  []docker.APIContainers;
	InuseImageIds     []string
}

func NewUndertakerData() *UndertakerData {
	return &UndertakerData{
		new([]RegexpHolder),
		new([]RegexpHolder),
		new([]RegexpHolder),
		new([]RegexpHolder),
		nil,
		make([]docker.APIContainers,0),
		make([]docker.APIContainers,0),
		make([]string,0)};
}

func (u UndertakerData) String() string {
	b, err := json.MarshalIndent(u,"","  ")
	if err != nil {
		return fmt.Sprintln(err)
	} 
	
	return fmt.Sprintln(string(b))
}

func generateRegexp(input *MultiStringOption) (*[]RegexpHolder) {
	var resptr = new([]RegexpHolder);
	for _,text := range *input {
		if pattern, err := regexp.Compile(text); err != nil {
			log.Fatal("invalid pattern: [", text, " ] - ", err)
		} else {
			*resptr = append(*resptr, RegexpHolder{pattern,text})
		}
	}
	return resptr;
}

func containsString(slice []string, val string) bool {
	for _, a := range slice {
		if a == val {
			return true
		}
	}
	return false
}

// loads exclude file removing comments and empty lines
func loadFile(filename string) (*MultiStringOption, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var res MultiStringOption;

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

		res.Set(text)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &res, nil
}



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

func processCommandLine() (*UndertakerData,error) {
	var container_excludes_file  string
	var image_excludes_file      string
	var conserving_deadline_secs int64
	var c_excludes               MultiStringOption
	var i_excludes               MultiStringOption
	var c_includes               MultiStringOption
	var i_includes               MultiStringOption

	flag.StringVar(&container_excludes_file, "filecexc", "", "read container excludes from file")
	flag.StringVar(&image_excludes_file,     "fileiexc", "", "read image excludes from file")
	flag.Int64Var(&conserving_deadline_secs, "wait", int64(3600), "conserving deadline in seconds")

	flag.Var(&c_excludes,"cexc","a single container exclude");
	flag.Var(&i_excludes,"iexc","a single image exclude");
	flag.Var(&c_includes,"cinc","a single container include");
	flag.Var(&i_includes,"iinc","a single image incclude");

	flag.Parse()

	var resptr = NewUndertakerData();

	if len(container_excludes_file) > 0 {
		ptr,err := loadFile(container_excludes_file);
		if err != nil {
			return nil,err
		}

		resptr.ContainerExc = generateRegexp(ptr);
	}

	if len(image_excludes_file) > 0 {
		ptr,err := loadFile(image_excludes_file);
		if err != nil {
			return nil,err
		}

		resptr.ImageExc = generateRegexp(ptr);
	}

	if len(c_excludes) > 0 {
		*resptr.ContainerExc = append(*resptr.ContainerExc,*generateRegexp(&c_excludes)...)
	}

	if len(i_excludes) > 0 {
		*resptr.ImageExc = append(*resptr.ImageExc,*generateRegexp(&i_excludes)...)
	}

	if len(c_includes) > 0 {
		*resptr.ContainerInc = append(*resptr.ContainerInc,*generateRegexp(&c_includes)...)
	}

	if len(i_includes) > 0 {
		*resptr.ImageInc = append(*resptr.ImageInc,*generateRegexp(&i_includes)...)
	}

	return resptr,nil
}


func main() {
	var dataPtr *UndertakerData
	var err     error

	dataPtr,err = processCommandLine();
	if err != nil {
		log.Fatal(err);
	}

	endpoint := "unix:///var/run/docker.sock"
	dataPtr.client, err = docker.NewClient(endpoint)
	if err != nil {
		log.Fatal(err)
	}

	dataPtr.ContainersAll, err = dataPtr.client.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		log.Fatal(err);
	}
	for _, cont := range dataPtr.ContainersAll {
		//fmt.Printf("%v %v\n",cont.Names,cont.Status);
		if strings.Index(cont.Status,"Exited") == 0 {
			dataPtr.ContainersExited = append(dataPtr.ContainersExited,cont);
		} else {
			// we need to inspect the container for the real ID (field 
			// APIContainers.Image may hold either a name or an ID)
			tmp, err := dataPtr.client.InspectContainer(cont.ID)
			if err != nil {
				log.Fatal(err)
			}
			if !containsString(dataPtr.InuseImageIds,tmp.Image)  {
				dataPtr.InuseImageIds = append(dataPtr.InuseImageIds,tmp.Image);
			}
		}
	}

	fmt.Println(dataPtr);

/*
	
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
*/
}

