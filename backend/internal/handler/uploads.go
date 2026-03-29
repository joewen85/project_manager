package handler

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type attachmentRequest struct {
	FileName     string `json:"fileName"`
	FilePath     string `json:"filePath"`
	RelativePath string `json:"relativePath"`
	FileSize     int64  `json:"fileSize"`
	MimeType     string `json:"mimeType"`
}

type uploadSourceFile struct {
	FileHeader   *multipart.FileHeader
	RelativePath string
}

func normalizeUploadPublicBase(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "/static/uploads"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return path.Clean(trimmed)
}

func normalizeAttachmentPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	normalized := path.Clean("/" + strings.TrimPrefix(trimmed, "/"))
	if normalized == "/" {
		return ""
	}
	return normalized
}

func normalizeRelativeUploadPath(value string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			continue
		}
		filtered = append(filtered, part)
	}
	if len(filtered) == 0 {
		return ""
	}
	return path.Join(filtered...)
}

func isAttachmentEmpty(item attachmentRequest) bool {
	return strings.TrimSpace(item.FileName) == "" &&
		strings.TrimSpace(item.FilePath) == "" &&
		strings.TrimSpace(item.MimeType) == "" &&
		item.FileSize == 0 &&
		strings.TrimSpace(item.RelativePath) == ""
}

func requestAttachments(single *attachmentRequest, many *[]attachmentRequest) ([]attachmentRequest, bool) {
	if many != nil {
		return *many, true
	}
	if single == nil {
		return nil, false
	}
	return []attachmentRequest{*single}, true
}

func validateAttachments(items []attachmentRequest, publicBase string) error {
	if len(items) == 0 {
		return nil
	}
	base := normalizeUploadPublicBase(publicBase)
	for _, item := range items {
		if isAttachmentEmpty(item) {
			continue
		}
		if item.FileSize < 0 {
			return errors.New("附件大小不能小于0")
		}
		filePath := normalizeAttachmentPath(item.FilePath)
		if filePath == "" {
			return errors.New("附件路径不能为空")
		}
		if filePath != base && !strings.HasPrefix(filePath, base+"/") {
			return errors.New("附件路径非法")
		}
	}
	return nil
}

func toModelAttachments(items []attachmentRequest) []model.Attachment {
	if len(items) == 0 {
		return []model.Attachment{}
	}
	result := make([]model.Attachment, 0, len(items))
	for _, item := range items {
		if isAttachmentEmpty(item) {
			continue
		}
		result = append(result, model.Attachment{
			FileName:     strings.TrimSpace(item.FileName),
			FilePath:     normalizeAttachmentPath(item.FilePath),
			RelativePath: normalizeRelativeUploadPath(item.RelativePath),
			FileSize:     item.FileSize,
			MimeType:     strings.TrimSpace(item.MimeType),
		})
	}
	return result
}

func firstModelAttachment(items []model.Attachment) model.Attachment {
	if len(items) == 0 {
		return model.Attachment{}
	}
	return items[0]
}

func sanitizeUploadFileName(fileName string) (dirPath string, baseName string, relativePath string) {
	normalized := normalizeRelativeUploadPath(fileName)
	if normalized == "" {
		return "", "upload.bin", "upload.bin"
	}
	baseName = path.Base(normalized)
	if baseName == "." || baseName == "/" || strings.TrimSpace(baseName) == "" {
		baseName = "upload.bin"
	}
	dirPath = path.Dir(normalized)
	if dirPath == "." {
		dirPath = ""
	}
	relativePath = baseName
	if dirPath != "" {
		relativePath = path.Join(dirPath, baseName)
	}
	return dirPath, baseName, relativePath
}

func (h *Handler) uploadDir() string {
	trimmed := strings.TrimSpace(h.Cfg.UploadDir)
	if trimmed == "" {
		return "./static/uploads"
	}
	return trimmed
}

func uploadDatePath(now time.Time) (string, string, string) {
	year := fmt.Sprintf("%04d", now.Year())
	month := fmt.Sprintf("%02d", int(now.Month()))
	day := fmt.Sprintf("%02d", now.Day())
	return year, month, day
}

func buildUploadPublicPath(base, year, month, day, relativePath string) string {
	publicPath := path.Join(normalizeUploadPublicBase(base), year, month, day, relativePath)
	if !strings.HasPrefix(publicPath, "/") {
		return "/" + publicPath
	}
	return publicPath
}

func topLevelFolder(relativePath string) string {
	normalized := normalizeRelativeUploadPath(relativePath)
	if normalized == "" {
		return ""
	}
	index := strings.Index(normalized, "/")
	if index <= 0 {
		return ""
	}
	return normalized[:index]
}

func sanitizeZipDisplayName(folderName string) string {
	normalized := normalizeRelativeUploadPath(folderName)
	if normalized == "" {
		return "folder.zip"
	}
	baseName := path.Base(normalized)
	if baseName == "." || baseName == "/" || strings.TrimSpace(baseName) == "" {
		return "folder.zip"
	}
	return baseName + ".zip"
}

func (h *Handler) saveAttachmentFile(fileHeader *multipart.FileHeader, relativePath string, now time.Time) (model.Attachment, error) {
	if fileHeader == nil {
		return model.Attachment{}, errors.New("文件不能为空")
	}
	if strings.TrimSpace(relativePath) == "" {
		relativePath = fileHeader.Filename
	}

	dirPath, baseName, normalizedRelativePath := sanitizeUploadFileName(relativePath)
	extension := strings.ToLower(filepath.Ext(baseName))
	if len(extension) > 12 {
		extension = extension[:12]
	}

	year, month, day := uploadDatePath(now)

	targetDir := filepath.Join(h.uploadDir(), year, month, day)
	if dirPath != "" {
		targetDir = filepath.Join(targetDir, filepath.FromSlash(dirPath))
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return model.Attachment{}, err
	}

	fileName := uuid.NewString() + extension
	filePath := filepath.Join(targetDir, fileName)
	source, err := fileHeader.Open()
	if err != nil {
		return model.Attachment{}, err
	}
	defer source.Close()

	destination, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return model.Attachment{}, err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		_ = os.Remove(filePath)
		return model.Attachment{}, err
	}

	publicPath := buildUploadPublicPath(h.Cfg.UploadPublicBase, year, month, day, path.Join(dirPath, fileName))

	return model.Attachment{
		FileName:     baseName,
		FilePath:     publicPath,
		RelativePath: normalizedRelativePath,
		FileSize:     fileHeader.Size,
		MimeType:     fileHeader.Header.Get("Content-Type"),
	}, nil
}

func (h *Handler) saveAttachmentZip(folderName string, files []uploadSourceFile, now time.Time) (model.Attachment, error) {
	if len(files) == 0 {
		return model.Attachment{}, errors.New("文件不能为空")
	}

	year, month, day := uploadDatePath(now)
	targetDir := filepath.Join(h.uploadDir(), year, month, day)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return model.Attachment{}, err
	}

	displayName := sanitizeZipDisplayName(folderName)
	storedName := uuid.NewString() + ".zip"
	zipPath := filepath.Join(targetDir, storedName)

	destination, err := os.OpenFile(zipPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return model.Attachment{}, err
	}
	writer := zip.NewWriter(destination)

	for _, fileHeader := range files {
		if fileHeader.FileHeader == nil {
			continue
		}
		_, baseName, relativePath := sanitizeUploadFileName(fileHeader.RelativePath)
		entryName := normalizeRelativeUploadPath(relativePath)
		if entryName == "" {
			entryName = path.Join(strings.TrimSuffix(displayName, ".zip"), baseName)
		}
		if !strings.HasPrefix(entryName, folderName+"/") && entryName != folderName {
			entryName = path.Join(folderName, baseName)
		}

		header := &zip.FileHeader{
			Name:   entryName,
			Method: zip.Deflate,
		}
		header.SetMode(0o644)

		entryWriter, createErr := writer.CreateHeader(header)
		if createErr != nil {
			_ = writer.Close()
			_ = destination.Close()
			_ = os.Remove(zipPath)
			return model.Attachment{}, createErr
		}

		source, openErr := fileHeader.FileHeader.Open()
		if openErr != nil {
			_ = writer.Close()
			_ = destination.Close()
			_ = os.Remove(zipPath)
			return model.Attachment{}, openErr
		}

		if _, copyErr := io.Copy(entryWriter, source); copyErr != nil {
			_ = source.Close()
			_ = writer.Close()
			_ = destination.Close()
			_ = os.Remove(zipPath)
			return model.Attachment{}, copyErr
		}
		_ = source.Close()
	}

	if err := writer.Close(); err != nil {
		_ = destination.Close()
		_ = os.Remove(zipPath)
		return model.Attachment{}, err
	}
	if err := destination.Close(); err != nil {
		_ = os.Remove(zipPath)
		return model.Attachment{}, err
	}

	info, err := os.Stat(zipPath)
	if err != nil {
		_ = os.Remove(zipPath)
		return model.Attachment{}, err
	}

	publicPath := buildUploadPublicPath(h.Cfg.UploadPublicBase, year, month, day, storedName)
	return model.Attachment{
		FileName:     displayName,
		FilePath:     publicPath,
		RelativePath: displayName,
		FileSize:     info.Size(),
		MimeType:     "application/zip",
	}, nil
}

func collectUploadFiles(c *gin.Context) ([]uploadSourceFile, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, err
	}
	files := make([]uploadSourceFile, 0)
	relativePaths := form.Value["relativePaths"]
	relativePathIndex := 0
	if headers, ok := form.File["files"]; ok && len(headers) > 0 {
		for _, header := range headers {
			relativePath := ""
			if relativePathIndex < len(relativePaths) {
				relativePath = relativePaths[relativePathIndex]
			}
			relativePathIndex += 1
			files = append(files, uploadSourceFile{
				FileHeader:   header,
				RelativePath: relativePath,
			})
		}
	}
	if headers, ok := form.File["file"]; ok && len(headers) > 0 {
		for _, header := range headers {
			relativePath := ""
			if relativePathIndex < len(relativePaths) {
				relativePath = relativePaths[relativePathIndex]
			}
			relativePathIndex += 1
			files = append(files, uploadSourceFile{
				FileHeader:   header,
				RelativePath: relativePath,
			})
		}
	}
	if len(files) == 0 {
		return nil, errors.New("请上传文件")
	}
	return files, nil
}

func (h *Handler) UploadFile(c *gin.Context) {
	files, err := collectUploadFiles(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, "UPLOAD_FILE_REQUIRED", "请上传文件")
		return
	}

	standaloneFiles := make([]uploadSourceFile, 0, len(files))
	folderGroups := make(map[string][]uploadSourceFile)
	folderOrder := make([]string, 0)
	for _, file := range files {
		if file.FileHeader == nil {
			continue
		}
		relativePath := file.RelativePath
		if strings.TrimSpace(relativePath) == "" {
			relativePath = file.FileHeader.Filename
		}
		_, _, relativePath = sanitizeUploadFileName(relativePath)
		folderName := topLevelFolder(relativePath)
		if folderName == "" {
			file.RelativePath = relativePath
			standaloneFiles = append(standaloneFiles, file)
			continue
		}
		if _, exists := folderGroups[folderName]; !exists {
			folderOrder = append(folderOrder, folderName)
		}
		file.RelativePath = relativePath
		folderGroups[folderName] = append(folderGroups[folderName], file)
	}

	now := time.Now()
	attachments := make([]model.Attachment, 0, len(standaloneFiles)+len(folderOrder))
	for _, file := range standaloneFiles {
		attachment, saveErr := h.saveAttachmentFile(file.FileHeader, file.RelativePath, now)
		if saveErr != nil {
			respondDBError(c, http.StatusBadRequest, "UPLOAD_FILE_FAILED", saveErr)
			return
		}
		attachments = append(attachments, attachment)
	}
	for _, folderName := range folderOrder {
		attachment, saveErr := h.saveAttachmentZip(folderName, folderGroups[folderName], now)
		if saveErr != nil {
			respondDBError(c, http.StatusBadRequest, "UPLOAD_FILE_FAILED", saveErr)
			return
		}
		attachments = append(attachments, attachment)
	}

	response := gin.H{"attachments": attachments}
	if len(attachments) == 1 {
		response["attachment"] = attachments[0]
	}
	c.JSON(http.StatusCreated, response)
}
