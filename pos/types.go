package main

// ─── Data Types ───────────────────────────────────────────────────

type Categoria struct {
	ID          int    `json:"id"`
	Nombre      string `json:"nombre"`
	Slug        string `json:"slug"`
	Icono       string `json:"icono"`
	Descripcion string `json:"descripcion"`
	Orden       int    `json:"orden"`
}

type Marca struct {
	ID     int    `json:"id"`
	Nombre string `json:"nombre"`
	LogoURL string `json:"logo_url"`
}

type Modelo struct {
	ID             int    `json:"id"`
	MarcaID        int    `json:"marca_id"`
	MarcaNombre    string `json:"marca_nombre"`
	Nombre         string `json:"nombre"`
	AñoLanzamiento int    `json:"año_lanzamiento"`
	Activo         bool   `json:"activo"`
	PiezasCount    int    `json:"piezas_count"`
}

type Pieza struct {
	ID                  int          `json:"id"`
	Codigo              string       `json:"codigo"`
	CategoriaID         int          `json:"categoria_id"`
	CategoriaNombre     string       `json:"categoria_nombre"`
	CategoriaSlug       string       `json:"categoria_slug"`
	Nombre              string       `json:"nombre"`
	Descripcion         string       `json:"descripcion"`
	Precio              float64      `json:"precio"`
	Costo               float64      `json:"costo"`
	Stock               int          `json:"stock"`
	StockMinimo         int          `json:"stock_minimo"`
	ImagenURL           string       `json:"imagen_url"`
	ImagenesAdicionales string       `json:"imagenes_adicionales"`
	Estado              string       `json:"estado"`
	GarantiaDias        int          `json:"garantia_dias"`
	Proveedor           string       `json:"proveedor"`
	Ubicacion           string       `json:"ubicacion"`
	Activa              bool         `json:"activa"`
	Compatibles         []Compatible `json:"compatibles"`
}

type Compatible struct {
	ModeloID     int    `json:"modelo_id"`
	ModeloNombre string `json:"modelo_nombre"`
	MarcaNombre  string `json:"marca_nombre"`
}

type PiezaResumen struct {
	ID              int     `json:"id"`
	Codigo          string  `json:"codigo,omitempty"`
	Nombre          string  `json:"nombre"`
	Precio          float64 `json:"precio"`
	Costo           float64 `json:"costo,omitempty"`
	Stock           int     `json:"stock"`
	StockMinimo     int     `json:"stock_minimo,omitempty"`
	ImagenURL       string  `json:"imagen_url,omitempty"`
	Estado          string  `json:"estado,omitempty"`
	CategoriaNombre string  `json:"categoria_nombre,omitempty"`
}

type Venta struct {
	ID             int     `json:"id"`
	Folio          int     `json:"folio"`
	ClienteNombre  string  `json:"cliente_nombre"`
	ClienteTelefono string `json:"cliente_telefono"`
	Total          float64 `json:"total"`
	FormaPago      string  `json:"forma_pago"`
	VendedorNombre string  `json:"vendedor_nombre"`
	CreadoEn       string  `json:"creado_en"`
}

type PiezaCount struct {
	Nombre string `json:"nombre"`
	Estado string `json:"estado,omitempty"`
	Count  int    `json:"count"`
}

type DashboardStats struct {
	TotalPiezas       int           `json:"total_piezas"`
	TotalModelos      int           `json:"total_modelos"`
	TotalMarcas       int           `json:"total_marcas"`
	StockBajo         int           `json:"stock_bajo"`
	ValorInventario   float64       `json:"valor_inventario"`
	GananciaPotencial float64       `json:"ganancia_potencial"`
	Agotadas          int           `json:"agotadas"`
	TotalVentas       int           `json:"total_ventas"`
	UltimasPiezas     []PiezaResumen `json:"ultimas_piezas"`
	StockBajoLista    []PiezaResumen `json:"stock_bajo_lista"`
	PiezasPorCategoria []PiezaCount  `json:"piezas_por_categoria"`
	PiezasPorEstado   []PiezaCount  `json:"piezas_por_estado"`
}
