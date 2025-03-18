package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	apiBase      = "http://localhost:8080"
	testEmail    = "test@example.com"
	testPassword = "password123"
)

type authResponse struct {
	Token string `json:"token"`
}

type fileResponse struct {
	Message string `json:"message"`
	File    struct {
		ID string `json:"id"`
	} `json:"file"`
}

type presignedResponse struct {
	PresignedURL string `json:"presigned_url"`
	ExpiresIn    string `json:"expires_in"`
}


// TestAPIEndpoints runs tests against the API endpoints
func TestAPIEndpoints(t *testing.T) {
	// Make sure the server is running
	t.Log("Ensuring API server is running...")
	resp, err := http.Get(apiBase)
	if err != nil {
		t.Fatalf("API server is not running at %s: %v", apiBase, err)
	}
	defer resp.Body.Close()

	// Register a new user
	t.Run("Register User", func(t *testing.T) {
		payload := map[string]interface{}{
			"email":    testEmail,
			"password": testPassword,
		}
		jsonPayload, _ := json.Marshal(payload)

		resp, err := http.Post(
			apiBase+"/auth/register",
			"application/json",
			bytes.NewBuffer(jsonPayload),
		)
		if err != nil {
			t.Fatalf("Failed to register user: %v", err)
		}
		defer resp.Body.Close()

		// We don't fail if user already exists
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Failed to register user. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
		}
	})

	// Login and get token
	var token string
	t.Run("Login", func(t *testing.T) {
		payload := map[string]string{
			"email":    testEmail,
			"password": testPassword,
		}
		jsonPayload, _ := json.Marshal(payload)

		resp, err := http.Post(
			apiBase+"/auth/login",
			"application/json",
			bytes.NewBuffer(jsonPayload),
		)
		if err != nil {
			t.Fatalf("Failed to login: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Failed to login. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
		}

		var authResp authResponse
		if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		token = authResp.Token
		if token == "" {
			t.Fatal("No token received")
		}
	})

	// Upload a file
	var fileID string
	t.Run("Upload File", func(t *testing.T) {
		if token == "" {
			t.Skip("Skipping test due to no auth token")
		}

		// Create a test file
		tempFile, err := os.CreateTemp("", "test-upload-*.txt")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		_, err = tempFile.Write([]byte("This is a test file for upload"))
		if err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tempFile.Close()

		// Create multipart request
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "test.txt")
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}

		fileContent, err := os.ReadFile(tempFile.Name())
		if err != nil {
			t.Fatalf("Failed to read temp file: %v", err)
		}
		_, err = part.Write(fileContent)
		if err != nil {
			t.Fatalf("Failed to write to form file: %v", err)
		}
		writer.Close()

		req, err := http.NewRequest("POST", apiBase+"/file/upload", body)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Failed to upload file. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
		}

		var fileResp fileResponse
		if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		fileID = fileResp.File.ID
		if fileID == "" {
			t.Fatal("No file ID received")
		}
		t.Logf("Uploaded file ID: %s", fileID)
	})

	// Generate a presigned URL
	t.Run("Generate Presigned URL", func(t *testing.T) {
		if token == "" || fileID == "" {
			t.Skip("Skipping test due to no auth token or file ID")
		}

		payload := map[string]interface{}{
			"token_type": "time-limited",
			"duration":   30,
		}
		jsonPayload, _ := json.Marshal(payload)

		req, err := http.NewRequest(
			"POST",
			fmt.Sprintf("%s/file/presigned/%s", apiBase, fileID),
			bytes.NewBuffer(jsonPayload),
		)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Failed to generate presigned URL. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
		}

		var presignedResp presignedResponse
		if err := json.NewDecoder(resp.Body).Decode(&presignedResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if presignedResp.PresignedURL == "" {
			t.Fatal("No presigned URL received")
		}
		t.Logf("Presigned URL: %s", presignedResp.PresignedURL)
	})

	// List files
	t.Run("List User Files", func(t *testing.T) {
		if token == "" {
			t.Skip("Skipping test due to no auth token")
		}

		req, err := http.NewRequest("GET", apiBase+"/file/list", nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Failed to list files. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
		}

		// Just check we can decode the response
		var files []interface{}
		if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		t.Logf("Found %d files", len(files))
	})

	// Get file metadata
	t.Run("Get File Metadata", func(t *testing.T) {
		if token == "" || fileID == "" {
			t.Skip("Skipping test due to no auth token or file ID")
		}

		req, err := http.NewRequest(
			"GET",
			fmt.Sprintf("%s/file/metadata/%s", apiBase, fileID),
			nil,
		)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Failed to get file metadata. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
		}
	})

	// Batch presigned URL generation
	t.Run("Generate Batch Presigned URLs", func(t *testing.T) {
		if token == "" || fileID == "" {
			t.Skip("Skipping test due to no auth token or file ID")
		}

		payload := map[string]interface{}{
			"file_ids":   []string{fileID},
			"token_type": "time-limited",
			"duration":   30,
		}
		jsonPayload, _ := json.Marshal(payload)

		req, err := http.NewRequest(
			"POST",
			apiBase+"/file/presigned",
			bytes.NewBuffer(jsonPayload),
		)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Failed to generate batch presigned URLs. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
		}
	})

	// Create a second file for batch deletion
	var secondFileID string
	t.Run("Upload Second File", func(t *testing.T) {
		if token == "" {
			t.Skip("Skipping test due to no auth token")
		}

		// Create test file content
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "test2.txt")
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}

		_, err = part.Write([]byte("Second test file content"))
		if err != nil {
			t.Fatalf("Failed to write to form file: %v", err)
		}
		writer.Close()

		req, err := http.NewRequest("POST", apiBase+"/file/upload", body)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Failed to upload file. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
		}

		var fileResp fileResponse
		if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		secondFileID = fileResp.File.ID
		if secondFileID == "" {
			t.Fatal("No file ID received for second file")
		}
		t.Logf("Uploaded second file ID: %s", secondFileID)
	})

	// Delete file
	t.Run("Delete File", func(t *testing.T) {
		if token == "" || fileID == "" {
			t.Skip("Skipping test due to no auth token or file ID")
		}

		req, err := http.NewRequest(
			"DELETE",
			fmt.Sprintf("%s/file/%s", apiBase, fileID),
			nil,
		)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Failed to delete file. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
		}

		t.Logf("Successfully deleted file: %s", fileID)

		// Verify file is deleted by trying to get metadata (should fail)
		verifyReq, err := http.NewRequest(
			"GET",
			fmt.Sprintf("%s/file/metadata/%s", apiBase, fileID),
			nil,
		)
		if err != nil {
			t.Fatalf("Failed to create verification request: %v", err)
		}
		verifyReq.Header.Set("Authorization", "Bearer "+token)

		verifyResp, err := client.Do(verifyReq)
		if err != nil {
			t.Fatalf("Failed to send verification request: %v", err)
		}
		defer verifyResp.Body.Close()

		// Should get a 404 not found
		if verifyResp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 status for deleted file, got %d", verifyResp.StatusCode)
		} else {
			t.Log("Verified file is deleted")
		}
	})

	// Test batch delete
	t.Run("Batch Delete Files", func(t *testing.T) {
		if token == "" || secondFileID == "" {
			t.Skip("Skipping test due to no auth token or second file ID")
		}

		payload := map[string]interface{}{
			"file_ids": []string{secondFileID},
		}
		jsonPayload, _ := json.Marshal(payload)

		req, err := http.NewRequest(
			"POST",
			apiBase+"/file/delete",
			bytes.NewBuffer(jsonPayload),
		)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to send request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Failed to batch delete files. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
		}

		// Parse response to check results
		var deleteResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&deleteResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		results, ok := deleteResp["results"].(map[string]interface{})
		if !ok {
			t.Fatal("Invalid response format for batch delete")
		}

		// Check result for our file
		result, exists := results[secondFileID].(string)
		if !exists || result != "Deleted successfully" {
			t.Errorf("Failed to delete file %s: %v", secondFileID, result)
		} else {
			t.Logf("Successfully batch deleted file: %s", secondFileID)
		}

		// Verify file is deleted by trying to get metadata (should fail)
		verifyReq, err := http.NewRequest(
			"GET",
			fmt.Sprintf("%s/file/metadata/%s", apiBase, secondFileID),
			nil,
		)
		if err != nil {
			t.Fatalf("Failed to create verification request: %v", err)
		}
		verifyReq.Header.Set("Authorization", "Bearer "+token)

		verifyResp, err := client.Do(verifyReq)
		if err != nil {
			t.Fatalf("Failed to send verification request: %v", err)
		}
		defer verifyResp.Body.Close()

		// Should get a 404 not found
		if verifyResp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 status for deleted file, got %d", verifyResp.StatusCode)
		} else {
			t.Log("Verified file is deleted")
		}
	})
}

func TestMain(m *testing.M) {
	// Wait for API server to be ready
	tries := 0
	for tries < 5 {
		resp, err := http.Get(apiBase)
		if err == nil {
			resp.Body.Close()
			break
		}
		fmt.Printf("Waiting for API server to be ready (attempt %d/5)...\n", tries+1)
		time.Sleep(2 * time.Second)
		tries++
	}

	// Run tests
	os.Exit(m.Run())
}
