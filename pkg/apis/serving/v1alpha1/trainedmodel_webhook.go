/*
Copyright 2021 The KServe Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"
	"regexp"
	"strings"
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
	"github.com/kserve/kserve/pkg/agent/storage"
	"github.com/kserve/kserve/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// regular expressions for validation of isvc name
const (
	CommaSpaceSeparator                 = ", "
	TmNameFmt                    string = "[a-zA-Z0-9_-]+"
	InvalidTmNameFormatError            = "the Trained Model \"%s\" is invalid: a Trained Model name must consist of alphanumeric characters, '_', or '-'. (e.g. \"my-Name\" or \"abc_123\", regex used for validation is '%s')"
	InvalidStorageUriFormatError        = "the Trained Model \"%s\" storageUri field is invalid. The storage uri must start with one of the prefixes: %s. (the storage uri given is \"%s\")"
	InvalidTmMemoryModification         = "the Trained Model \"%s\" memory field is immutable. The memory was \"%s\" but it is updated to \"%s\""
	InvalidIsvcNameError = "the inferenceservice \"%s\" specified in the Trained Model \"%s\" does not exist."
)

var (
	// log is for logging in this package.
	tmLogger = logf.Log.WithName("trainedmodel-alpha1-validator")
	// regular expressions for validation of tm name
	TmRegexp = regexp.MustCompile("^" + TmNameFmt + "$")
	// protocols that are accepted by storage uri
	StorageUriProtocols = strings.Join(storage.GetAllProtocol(), CommaSpaceSeparator)
)

// +kubebuilder:webhook:verbs=create;update,path=/validate-trainedmodel,mutating=false,failurePolicy=fail,groups=serving.kserve.io,resources=trainedmodels,versions=v1alpha1,name=trainedmodel.kserve-webhook-server.validator

var _ webhook.Validator = &TrainedModel{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (tm *TrainedModel) ValidateCreate() error {
	tmLogger.Info("validate create", "name", tm.Name)
	return utils.FirstNonNilError([]error{
		tm.validateTrainedModel(),
	})
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (tm *TrainedModel) ValidateUpdate(old runtime.Object) error {
	tmLogger.Info("validate update", "name", tm.Name)
	oldTm := convertToTrainedModel(old)

	return utils.FirstNonNilError([]error{
		tm.validateTrainedModel(),
		tm.validateMemorySpecNotModified(oldTm),
	})
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (tm *TrainedModel) ValidateDelete() error {
	tmLogger.Info("validate delete", "name", tm.Name)
	return nil
}

// Validates ModelSpec memory is not modified from previous TrainedModel state
func (tm *TrainedModel) validateMemorySpecNotModified(oldTm *TrainedModel) error {
	newTmMemory := tm.Spec.Model.Memory
	oldTmMemory := oldTm.Spec.Model.Memory
	if !newTmMemory.Equal(oldTmMemory) {
		return fmt.Errorf(InvalidTmMemoryModification, tm.Name, oldTmMemory.String(), newTmMemory.String())
	}
	return nil
}

// Validates format of TrainedModel's fields
func (tm *TrainedModel) validateTrainedModel() error {
	return utils.FirstNonNilError([]error{
		tm.validateTrainedModelName(),
		tm.validateStorageURI(),
		tm.validateIsvcName(),
	})
}

// Convert runtime.Object into TrainedModel
func convertToTrainedModel(old runtime.Object) *TrainedModel {
	tm := old.(*TrainedModel)
	return tm
}

// Validates format for TrainedModel's name
func (tm *TrainedModel) validateTrainedModelName() error {
	if !TmRegexp.MatchString(tm.Name) {
		return fmt.Errorf(InvalidTmNameFormatError, tm.Name, TmRegexp)
	}
	return nil
}

// Trainedmodel에 명시된 isvc가 존재하는지 체크
func (tm *TrainedModel) validateIsvcName() error {
	found, err := CheckServicesStartingWithIsvcName(tm.Namespace, tm.Spec.InferenceService)
	if err != nil {
		return fmt.Errorf("Error: %v\n", err)
	}
	if found {
		return nil
	} else {
		return fmt.Errorf(InvalidIsvcNameError, tm.Spec.InferenceService, tm.Name)
	}	
	return nil
}




func CheckServicesStartingWithIsvcName(namespace string, isvcname string) (bool, error) {
    // Load the Kubernetes configuration
	/*
	cfg, err := k8sconfig.GetConfig()
	if err != nil {
		return fmt.Errorf(err, "")
	}
	*/
	
    config, err := clientcmd.BuildConfigFromFlags("", "")
    if err != nil {
        return false, err
    }

    // Create a Kubernetes API clientset
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return false, err
    }

    // Get list of services in the namespace.
	services, err := clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	svcname := isvcname + "-" + "predictor"
	// Check if any services start with the prefix.
	for _, service := range services.Items {
		if strings.HasPrefix(service.Name, svcname) {
			return true, nil
		}
	}

	// No services found.
	return false, nil
}

// Validates TrainModel's storageURI
func (tm *TrainedModel) validateStorageURI() error {
	if !utils.IsPrefixSupported(tm.Spec.Model.StorageURI, storage.GetAllProtocol()) {
		return fmt.Errorf(InvalidStorageUriFormatError, tm.Name, StorageUriProtocols, tm.Spec.Model.StorageURI)
	}
	return nil
}
