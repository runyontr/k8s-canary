package service

import (
	"bufio"
	"github.com/runyontr/k8s-canary/app/models"
	"io"
	"os"
	"strings"
	"errors"
)

type AppInfoService interface {
	GetAppInfo() (models.AppInfo, error)
}


func New(version int) (AppInfoService, error) {

	switch version {
	case 1:
		return &appInfoBaseline{}, nil
	case 2:
		return &appInfoBroken{}, nil
	case 3:
		return &appInfoWithNamespace{}, nil
	}
	//Make sure in Kubernetes

	return nil, errors.New("unknown version request")
}


//appInfoBaseline is the implementation of the AppInfoService interface.  This implementation has a bug where
// the Namespace value is not populated.
type appInfoBaseline struct {
}

//GetAppInfo returns the app info of the running application
func (s *appInfoBaseline) GetAppInfo() (models.AppInfo, error) {

	info := models.AppInfo{}
	info.Labels = make(map[string]string)

	info.PodName = os.Getenv("MY_POD_NAME") //custom defined in the deployment spec

	file, err := os.Open("/etc/labels")
	if err != nil {
		return info, err
	}
	defer file.Close()

	//overkill, but read it fresh each time
	reader := bufio.NewReader(file)

	for {
		line, err := reader.ReadString('\n')

		// check if the line has = sign
		// and process the line. Ignore the rest.
		if equal := strings.Index(line, "="); equal >= 0 {
			if key := strings.TrimSpace(line[:equal]); len(key) > 0 {
				value := ""
				if len(line) > equal {
					value = strings.TrimSpace(line[equal+1:])
				}

				value = strings.Replace(value, "\"", "", -1)
				switch key {
				case "app":
					info.AppName = value
				case "release":
					info.Release = value
				default:
					info.Labels[key] = value
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return info, err
		}
	}

	return info, err
}


//appInfoBroken is an implementation of the AppInfoService that always returns an error
type appInfoBroken struct{

}

//GetAppInfo should returns the app info of the running application.  This implementation will always produce an error.
func (s *appInfoBroken) GetAppInfo() (models.AppInfo, error) {

	return models.AppInfo{}, errors.New("something went wrong")
}


//appInfoWithNamespace is the implementation of the AppInfoService interface that has the bug from appInfoBaseline fixed.
type appInfoWithNamespace struct{

}

//GetAppInfo returns the app info of the running pod
func (s *appInfoWithNamespace) GetAppInfo() (models.AppInfo, error) {

	info := models.AppInfo{}
	info.Labels = make(map[string]string)

	info.PodName = os.Getenv("MY_POD_NAME") //custom defined in the deployment spec
	//insert fix
	info.Namespace = os.Getenv("MY_POD_NAMESPACE") //custom defined in the deployment spec

	//the /etc/labels file is present because of the podInfo volume defined in the deployment yaml
	file, err := os.Open("/etc/labels")
	if err != nil {
		return info, err
	}
	defer file.Close()

	//Reading it fresh each time will allow us to obtain the updated label values
	// as they're added/removed from the pod.
	reader := bufio.NewReader(file)

	for {
		line, err := reader.ReadString('\n')

		// check if the line has = sign
		// and process the line. Ignore the rest.
		if equal := strings.Index(line, "="); equal >= 0 {
			if key := strings.TrimSpace(line[:equal]); len(key) > 0 {
				value := ""
				if len(line) > equal {
					value = strings.TrimSpace(line[equal+1:])
				}

				//Make '\"appinfo\"' map to 'appinfo'
				value = strings.Replace(value, "\"", "", -1)
				switch key {
				case "app":
					info.AppName = value
				case "release":
					info.Release = value
				default:
					info.Labels[key] = value
				}
			}
		}
		//done reading
		if err == io.EOF {
			break
		}
		//Something went wrong, pass the error object back
		if err != nil {
			return info, err
		}
	}
	//Don't force an error return
	return info, err
}
