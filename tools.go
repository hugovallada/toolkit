package toolkit

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	randomStringSource = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVXYZ0123456789_+"
)

// Tools is the type used to instantiate this module. Any variable of this type will have access
// to all the methods with the reciever *Tools
type Tools struct {
	MaxFileSize      int
	AllowedFileTypes []string
}

/**
 * Cria 2 variáveis s, que é um slice de runes com o tamanho passado e r que será um slice de runes baseado na string passada
 * Itera sobre o s, pegando apenas o index
 * p recebe um inteiro randomico
 * x recebe p como um uint64 e y recebe o tamanho de r como um uint64
 * s recebe no seu index a rune que está no slice r na posição resultante do calculo de módulos
 * retorna o slice de runes s como uma string
 */
// RandomString returns a string of random characters of lenght n, using randomStringSource
// as the source for the string
func (t *Tools) RandomString(size int) string {
	s, r := make([]rune, size), []rune(randomStringSource)

	for i := range s {
		p, _ := rand.Prime(rand.Reader, len(r))
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}

	return string(s)
}

// UploadedFile is a struct used to save information about an uploaded file
type UploadedFile struct {
	NewFileName      string
	OriginalFileName string
	FileSize         int64
}

type UploadFilesParams struct {
	uploadedFiles []*UploadedFile
	header        *multipart.FileHeader
	t             *Tools
	renameFile    bool
	uploadDir     string
}

func uploadFiles(ufp UploadFilesParams) ([]*UploadedFile, error) {
	uploadedFiles, header, t, renameFile, uploadDir := ufp.uploadedFiles, ufp.header, ufp.t, ufp.renameFile, ufp.uploadDir

	var uploadedFile UploadedFile
	inFile, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer inFile.Close()

	buff := make([]byte, 512)
	_, err = inFile.Read(buff)
	if err != nil {
		return nil, err
	}

	//check to see if the file type is permitted
	allowed := false
	fileType := http.DetectContentType(buff)

	if len(t.AllowedFileTypes) > 0 {
		for _, typeReceived := range t.AllowedFileTypes {
			if strings.EqualFold(fileType, typeReceived) {
				allowed = true
			}
		}
	} else {
		allowed = true
	}

	if !allowed {
		return nil, errors.New("the uploaded file type is not permitted")
	}

	_, err = inFile.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	uploadedFile.OriginalFileName = header.Filename

	if renameFile {
		uploadedFile.NewFileName = fmt.Sprintf("%s%s", t.RandomString(25), filepath.Ext(header.Filename))
	} else {
		uploadedFile.NewFileName = header.Filename
	}

	var outfile *os.File
	defer outfile.Close()

	if outfile, err = os.Create(filepath.Join(uploadDir, uploadedFile.NewFileName)); err != nil {
		return nil, err
	} else {
		fileSize, err := io.Copy(outfile, inFile)
		if err != nil {
			return nil, err
		}
		uploadedFile.FileSize = fileSize
	}

	uploadedFiles = append(uploadedFiles, &uploadedFile)
	return uploadedFiles, nil
}

func (t *Tools) UploadFiles(r *http.Request, uploadDir string, rename ...bool) ([]*UploadedFile, error) {
	renameFile := shouldRenameFile(rename...)

	var uploadedFiles []*UploadedFile

	if t.MaxFileSize == 0 {
		t.MaxFileSize = int(math.Pow(1024, 3))
	}

	err := t.CreateDirIfNotExists(uploadDir)

	if err != nil {
		return nil, err
	}

	err = r.ParseMultipartForm(int64(t.MaxFileSize))

	if err != nil {
		return nil, errors.New("the uploaded file is too big")
	}

	for _, fHeaders := range r.MultipartForm.File {
		for _, header := range fHeaders {
			uploadedFiles, err = uploadFiles(UploadFilesParams{uploadedFiles, header, t, renameFile, uploadDir})
			if err != nil {
				return uploadedFiles, err
			}
		}
	}
	return uploadedFiles, nil
}

func (t *Tools) UploadOneFile(r *http.Request, uploadDir string, rename ...bool) (*UploadedFile, error) {
	renameFile := shouldRenameFile(rename...)

	files, err := t.UploadFiles(r, uploadDir, renameFile)

	if err != nil {
		return nil, err
	}

	return files[0], nil
}

// CreateDirIfNotExists create a directory and all necessary parents if it does not exists
func (t *Tools) CreateDirIfNotExists(path string) error {
	const mode = 0755

	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, mode)
		if err != nil {
			return err
		}
	}
	return nil
}

// Slugify is a mean of creating a slug from a string
func (t *Tools) Slugify(s string) (string, error) {
	if s == "" {
		return "", errors.New("empty string not permitted")
	}

	var re = regexp.MustCompile(`[^a-z\d]+`)

	slug := strings.Trim(re.ReplaceAllString(strings.ToLower(s), "-"), "-")

	if len(slug) == 0 {
		return "", errors.New("after removing characters, slug is zero length")
	}
	return slug, nil
}

// DownloadsStatic File downloads a file and tries to force the browser to avoid displaying it in the browser window by setting content disposition.
// It also alllows specification of the display name
func (t *Tools) DownloadStaticFile(w http.ResponseWriter, r *http.Request, pathName, fileName, displayName string) {
	fp := path.Join(pathName, fileName)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", displayName))

	http.ServeFile(w, r, fp)
}

func shouldRenameFile(rename ...bool) bool {
	if len(rename) > 0 {
		return rename[0]
	}
	return true
}
