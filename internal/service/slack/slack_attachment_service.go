package slack

import (
	"fmt"
	"github.com/sharovik/devbot/internal/container"
	"github.com/sharovik/devbot/internal/dto"
	"github.com/sharovik/devbot/internal/helper"
	"github.com/sharovik/devbot/internal/log"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
)

const (
	zipFileType           = "zip"
	defaultResultFilename = "result.zip"
)

var supportedFileTypes = map[string]string{
	zipFileType: zipFileType,
}

func isValidFile(fileType string) bool {
	return supportedFileTypes[fileType] != ""
}

func validateFiles(files []dto.File) (dto.File, error) {
	for _, file := range files {
		if !isValidFile(file.Filetype) {
			err := fmt.Errorf("Wrong file type ")
			log.Logger().AddError(err).Interface("file", file).Msg("Wrong file type")
			return file, err
		}
	}

	return dto.File{}, nil
}

func processFile(channel string, file dto.File) (dto.File, error) {
	log.Logger().Debug().
		Str("url", file.URLPrivate).
		Msg("Start processing file")

	//First we need to download the file
	tmpFile, err := downloadFile(file.URLPrivate)
	if err != nil {
		return file, err
	}

	//Now we need to unzip the file and save the destination folder path
	var (
		src         = os.TempDir() + file.ID
		pathToFiles = src + "/downloaded_template"
	)
	_, err = helper.Unzip(tmpFile.Name(), pathToFiles)
	if err != nil {
		return file, err
	}

	log.Logger().Debug().Str("template_dir", pathToFiles).Msg("Template dir generated")

	//We run the command which compiles the template.
	//This will create in src 2 directories: one is for template html preview and second one for template
	cmd := exec.Command("./scripts/themer/themer.phar", fmt.Sprintf("--path=%s", pathToFiles))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Logger().AddError(err).
			Interface("file", file).
			Msg("Failed generate template")
		return file, err
	}

	//Now we need to remove the downloaded dir and zip the contains of src directory
	if err := deleteSrc(pathToFiles); err != nil {
		return file, err
	}

	resultFilePath := src + fmt.Sprintf("/%s", defaultResultFilename)
	if err := helper.Zip(src, resultFilePath); err != nil {
		return file, err
	}

	log.Logger().Debug().Str("result_zip_path", src+"/result.zip").Msg("Zip file created")

	if _, _, err := container.C.SlackClient.AttachFileTo(channel, resultFilePath, defaultResultFilename); err != nil {
		return file, err
	}

	if err := deleteSrc(src); err != nil {
		return file, err
	}

	return file, nil
}

func downloadFile(url string) (*os.File, error) {
	// Get the data
	resp, _, err := container.C.SlackClient.Request(http.MethodGet, url, []byte(``))
	if err != nil {
		return nil, err
	}

	// Create the file
	tmpFile, err := ioutil.TempFile(os.TempDir(), "devbot-*.zip")
	if err != nil {
		return nil, err
	}

	if _, err = tmpFile.Write(resp); err != nil {
		return nil, err
	}

	// Close the file
	if err := tmpFile.Close(); err != nil {
		return nil, err
	}

	return tmpFile, nil
}

func deleteSrc(src string) error {
	return os.RemoveAll(src)
}

func processFiles(message *dto.SlackResponseEventMessage) (dto.File, error) {
	log.Logger().Debug().
		Interface("files", message.Files).
		Msg("Files received")

	file, err := validateFiles(message.Files)
	if err != nil {
		return file, err
	}

	for _, fileReceived := range message.Files {
		file, err := processFile(message.Channel, fileReceived)
		if err != nil {
			return file, err
		}
	}

	return dto.File{}, nil
}
