package models

type VideoUploadMessage struct {
	VideoID        string `json:"video_id"`
	FileName       string `json:"file_name"`
	UploadFilePath string `json:"upload_file_path"`
	OriginalFormat string `json:"original_format"`
	UploaderID     string `json:"uploader_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
}
