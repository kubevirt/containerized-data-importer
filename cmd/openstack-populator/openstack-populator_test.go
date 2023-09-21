package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func setupMockServer() (*httptest.Server, string, int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, "", 0, err
	}

	mux := http.NewServeMux()

	port := listener.Addr().(*net.TCPAddr).Port
	identityServerURL := fmt.Sprintf("http://localhost:%d", port)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf(`{
            "versions": {
                "values": [
                    {
                        "id": "v3.0",
                        "links": [
                            {"rel": "self", "href": "%s/v3/"}
                        ],
                        "status": "stable"
                    }
                ]
            }
        }`, identityServerURL)
		fmt.Fprint(w, response)
	})

	mux.HandleFunc("/v2/images/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `mock_data`)
	})

	mux.HandleFunc("/v3/auth/tokens", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Subject-Token", "MIIFvgY")
		w.WriteHeader(http.StatusCreated)
		response := `{
            "token": {
                "methods": ["password"],
                "project": {
                    "domain": {
                        "id": "default",
                        "name": "Default"
                    },
                    "id": "8538a3f13f9541b28c2620eb19065e45",
                    "name": "admin"
                },
				"catalog": [
				{
					"type": "image",
                    "name": "glance",
							"endpoints": [{
								"url": "http://localhost:%d/v2/images",
								"region": "RegionOne",
								"interface": "public",
								"id": "29beb2f1567642eb810b042b6719ea88"

							}]
						}],
                "user": {
                    "domain": {
                        "id": "default",
                        "name": "Default"
                    },
                    "id": "3ec3164f750146be97f21559ee4d9c51",
                    "name": "admin"
                },
                "issued_at": "201406-10T20:55:16.806027Z"
            }
        }`
		response = fmt.Sprintf(response, port)
		fmt.Fprint(w, response)
	})

	server := httptest.NewUnstartedServer(mux)
	server.Listener = listener

	server.Start()

	return server, identityServerURL, port, nil
}

func TestPopulate(t *testing.T) {
	os.Setenv("username", "testuser")
	os.Setenv("password", "testpassword")
	os.Setenv("projectID", "testproject")
	os.Setenv("domainName", "testdomain")

	server, identityServerURL, port, err := setupMockServer()
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer server.Close()

	fmt.Printf("Mock server running on port: %d\n", port)

	fileName := "disk.img"
	secretName := "test-secret"
	imageID := "test-image-id"
	ownerUID := "test-uid"
	config := &appConfig{
		identityEndpoint: identityServerURL,
		secretName:       secretName,
		imageID:          imageID,
		ownerUID:         ownerUID,
		pvcSize:          100,
		volumePath:       fileName,
	}

	populate(config)

	file, err := os.Open(fileName)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != "mock_data\n" {
		t.Errorf("Expected %s, got %s", "mock_data\n", string(content))
	}

	err = os.Remove(fileName)
	if err != nil {
		t.Errorf("Failed to delete file: %v", err)
	}
}
