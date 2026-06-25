package config

import "testing"

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid json/info",
			cfg:     Config{FolderID: "folder", AuthKeyFile: "/key.json", LogLevel: "info", LogFormat: "json"},
			wantErr: false,
		},
		{
			name:    "valid text/debug",
			cfg:     Config{FolderID: "folder", AuthKeyFile: "/key.json", LogLevel: "debug", LogFormat: "text"},
			wantErr: false,
		},
		{
			name:    "missing folder id",
			cfg:     Config{AuthKeyFile: "/key.json", LogLevel: "info", LogFormat: "json"},
			wantErr: true,
		},
		{
			name:    "missing auth key file",
			cfg:     Config{FolderID: "folder", LogLevel: "info", LogFormat: "json"},
			wantErr: true,
		},
		{
			name:    "invalid log level",
			cfg:     Config{FolderID: "folder", AuthKeyFile: "/key.json", LogLevel: "verbose", LogFormat: "json"},
			wantErr: true,
		},
		{
			name:    "invalid log format",
			cfg:     Config{FolderID: "folder", AuthKeyFile: "/key.json", LogLevel: "info", LogFormat: "xml"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(&tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
