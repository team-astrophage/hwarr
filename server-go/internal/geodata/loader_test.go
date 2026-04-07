package geodata

import (
	"testing"
)

// sampleGeoJSON is a minimal FeatureCollection with two polygon features
// representing simplified regions for testing.
var sampleGeoJSON = []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": { "adm_nm": "서울특별시 강남구 역삼1동" },
      "geometry": {
        "type": "Polygon",
        "coordinates": [[[127.03, 37.49], [127.05, 37.49], [127.05, 37.51], [127.03, 37.51], [127.03, 37.49]]]
      }
    },
    {
      "type": "Feature",
      "properties": { "adm_nm": "서울특별시 종로구 청운효자동" },
      "geometry": {
        "type": "Polygon",
        "coordinates": [[[126.97, 37.58], [126.98, 37.58], [126.98, 37.59], [126.97, 37.59], [126.97, 37.58]]]
      }
    }
  ]
}`)

func TestLoadFromBytes(t *testing.T) {
	resolver, err := LoadFromBytes(sampleGeoJSON)
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}
	if resolver.RegionCount() != 2 {
		t.Errorf("expected 2 regions, got %d", resolver.RegionCount())
	}
}

func TestResolve_InsidePolygon(t *testing.T) {
	resolver, err := LoadFromBytes(sampleGeoJSON)
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	// Point inside 역삼1동 polygon
	label := resolver.Resolve(37.50, 127.04)
	if label != "서울특별시 강남구 역삼1동" {
		t.Errorf("expected '서울특별시 강남구 역삼1동', got '%s'", label)
	}

	// Point inside 청운효자동 polygon
	label = resolver.Resolve(37.585, 126.975)
	if label != "서울특별시 종로구 청운효자동" {
		t.Errorf("expected '서울특별시 종로구 청운효자동', got '%s'", label)
	}
}

func TestResolve_OutsideAllPolygons(t *testing.T) {
	resolver, err := LoadFromBytes(sampleGeoJSON)
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	// Point outside all polygons (somewhere in Busan)
	label := resolver.Resolve(35.15, 129.05)
	if label != "" {
		t.Errorf("expected empty string for out-of-bounds point, got '%s'", label)
	}
}

func TestResolve_MultiPolygon(t *testing.T) {
	multiPolyJSON := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": { "name": "다도해 해역" },
      "geometry": {
        "type": "MultiPolygon",
        "coordinates": [
          [[[126.0, 34.0], [126.1, 34.0], [126.1, 34.1], [126.0, 34.1], [126.0, 34.0]]],
          [[[126.2, 34.2], [126.3, 34.2], [126.3, 34.3], [126.2, 34.3], [126.2, 34.2]]]
        ]
      }
    }
  ]
}`)

	resolver, err := LoadFromBytes(multiPolyJSON)
	if err != nil {
		t.Fatalf("LoadFromBytes failed: %v", err)
	}

	// Point in first polygon
	label := resolver.Resolve(34.05, 126.05)
	if label != "다도해 해역" {
		t.Errorf("expected '다도해 해역', got '%s'", label)
	}

	// Point in second polygon
	label = resolver.Resolve(34.25, 126.25)
	if label != "다도해 해역" {
		t.Errorf("expected '다도해 해역', got '%s'", label)
	}

	// Point between polygons
	label = resolver.Resolve(34.15, 126.15)
	if label != "" {
		t.Errorf("expected empty string between polygons, got '%s'", label)
	}
}

func TestLoadFromBytes_EmptyFeatures(t *testing.T) {
	emptyJSON := []byte(`{"type": "FeatureCollection", "features": []}`)
	resolver, err := LoadFromBytes(emptyJSON)
	if err == nil {
		t.Error("expected error for empty features, got nil")
	}
	if resolver != nil {
		t.Error("expected nil resolver for empty features")
	}
}

func TestLoadFromBytes_InvalidJSON(t *testing.T) {
	_, err := LoadFromBytes([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadFromBytes_MissingName(t *testing.T) {
	noNameJSON := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "properties": { "other_field": "value" },
      "geometry": {
        "type": "Polygon",
        "coordinates": [[[127.0, 37.0], [127.1, 37.0], [127.1, 37.1], [127.0, 37.1], [127.0, 37.0]]]
      }
    }
  ]
}`)

	_, err := LoadFromBytes(noNameJSON)
	if err == nil {
		t.Error("expected error when no features have valid names")
	}
}

func TestLoadOrNil_MissingFile(t *testing.T) {
	resolver := LoadOrNil("/nonexistent/path/to/file.geojson")
	if resolver != nil {
		t.Error("expected nil for missing file")
	}
}

func TestExtractName_PriorityOrder(t *testing.T) {
	tests := []struct {
		name  string
		props map[string]interface{}
		want  string
	}{
		{
			name:  "adm_nm takes priority",
			props: map[string]interface{}{"adm_nm": "서울", "name": "대전"},
			want:  "서울",
		},
		{
			name:  "ADM_NM fallback",
			props: map[string]interface{}{"ADM_NM": "부산"},
			want:  "부산",
		},
		{
			name:  "EMD_KOR_NM fallback",
			props: map[string]interface{}{"EMD_KOR_NM": "대구"},
			want:  "대구",
		},
		{
			name:  "name last resort",
			props: map[string]interface{}{"name": "광주"},
			want:  "광주",
		},
		{
			name:  "empty string ignored",
			props: map[string]interface{}{"adm_nm": "", "name": "인천"},
			want:  "인천",
		},
		{
			name:  "whitespace only ignored",
			props: map[string]interface{}{"adm_nm": "  ", "name": "제주"},
			want:  "제주",
		},
		{
			name:  "no matching fields",
			props: map[string]interface{}{"foo": "bar"},
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractName(tc.props)
			if got != tc.want {
				t.Errorf("extractName(%v) = %q, want %q", tc.props, got, tc.want)
			}
		})
	}
}

func TestShortenName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"서울특별시 강남구 역삼1동", "서울특별시 강남구 역삼1동"},
		{"서울특별시  강남구  역삼1동", "서울특별시 강남구 역삼1동"},
		{"  서울특별시   강남구 ", "서울특별시 강남구"},
	}

	for _, tc := range tests {
		got := shortenName(tc.input)
		if got != tc.want {
			t.Errorf("shortenName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func BenchmarkResolve(b *testing.B) {
	resolver, err := LoadFromBytes(sampleGeoJSON)
	if err != nil {
		b.Fatalf("LoadFromBytes failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolver.Resolve(37.50, 127.04)
	}
}
