package models


//AppInfo holds runtime information about the application
type AppInfo struct {
	//PodName contains the name of the pod
	PodName string
	//AppName contains the name of the app.  This value is populated via the app tag
	AppName string
	//Namespace contains the namespace of where the pod is running
	Namespace string
	//Release contains the value of the Release tag.  Used to differentiate stable vs canary
	Release string
	//Labels contains all the other label key/values on the pod
	Labels  map[string]string //Contains other labels for app
}
