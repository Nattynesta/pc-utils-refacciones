package main

// ─── Status Labels & Colors ───────────────────────────────────────

var StatusLabels = map[string]string{
	"recibido":       "Recibido",
	"diagnosticando": "Diagnosticando",
	"presupuestado":  "Presupuestado",
	"aprobado":       "Aprobado",
	"reparando":      "Reparando",
	"reparado":       "Reparado",
	"entregado":      "Entregado",
	"cancelado":      "Cancelado",
}

var StatusColors = map[string]string{
	"recibido":       "primary",
	"diagnosticando": "warning",
	"presupuestado":  "secondary",
	"aprobado":       "info",
	"reparando":      "info",
	"reparado":       "success",
	"entregado":      "success",
	"cancelado":      "danger",
}

var StatusIcons = map[string]string{
	"recibido":       "box-seam",
	"diagnosticando": "search",
	"presupuestado":  "cash-coin",
	"aprobado":       "check2-circle",
	"reparando":      "tools",
	"reparado":       "check2-all",
	"entregado":      "hand-thumbs-up",
	"cancelado":      "x-circle",
}

var StatusSequence = []string{
	"recibido", "diagnosticando", "presupuestado",
	"aprobado", "reparando", "reparado", "entregado",
}

var TipoEquipoIcons = map[string]string{
	"laptop":  "laptop",
	"pc":      "pc-display",
	"celular": "phone",
	"audio":   "speaker",
	"otro":    "box",
}

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

type TipoEquipoOption struct {
	Value string
	Label string
	Icon  string
}

var TipoEquipos = []TipoEquipoOption{
	{"laptop", "Laptop", "laptop"},
	{"pc", "PC Escritorio", "pc-display"},
	{"celular", "Celular", "phone"},
	{"audio", "Audio", "speaker"},
	{"otro", "Otro", "box"},
}

// ─── Reparaciones ─────────────────────────────────────────────────

type Reparacion struct {
	ID                int     `json:"id"`
	Folio             int     `json:"folio"`
	Token             string  `json:"token"`
	TipoEquipo        string  `json:"tipo_equipo"`
	MarcaID           int     `json:"marca_id"`
	MarcaNombre       string  `json:"marca_nombre"`
	ModeloTexto       string  `json:"modelo_texto"`
	NumeroSerie       string  `json:"numero_serie"`
	IMEI              string  `json:"imei"`
	PasswordEquipo    string  `json:"password_equipo"`
	CondicionFisica   string  `json:"condicion_fisica"`
	ClienteNombre     string  `json:"cliente_nombre"`
	ClienteTelefono   string  `json:"cliente_telefono"`
	ClienteEmail      string  `json:"cliente_email"`
	FallaReportada    string  `json:"falla_reportada"`
	Diagnostico       string  `json:"diagnostico"`
	Accesorios        string  `json:"accesorios"`
	NotasCliente      string  `json:"notas_cliente"`
	NotasInternas     string  `json:"notas_internas"`
	Status            string  `json:"status"`
	FechaIngreso      string  `json:"fecha_ingreso"`
	FechaPrometida    string  `json:"fecha_prometida"`
	FechaEntrega      string  `json:"fecha_entrega"`
	CostoDiagnostico  float64 `json:"costo_diagnostico"`
	CostoReparacion   float64 `json:"costo_reparacion"`
	Total             float64 `json:"total"`
	Anticipo          float64 `json:"anticipo"`
	TecnicoID         int     `json:"tecnico_id"`
	TecnicoNombre     string  `json:"tecnico_nombre"`
	Activo            bool    `json:"activo"`
	DiasEnStatus      int     `json:"dias_en_status"`
	Historial         []ReparacionHistorial `json:"historial"`
	PiezasUsadas      []ReparacionPieza     `json:"piezas_usadas"`
	Archivos          []ReparacionArchivo   `json:"archivos"`
}

type ReparacionResumen struct {
	ID              int     `json:"id"`
	Folio           int     `json:"folio"`
	Token           string  `json:"token"`
	TipoEquipo      string  `json:"tipo_equipo"`
	MarcaNombre     string  `json:"marca_nombre"`
	ModeloTexto     string  `json:"modelo_texto"`
	ClienteNombre   string  `json:"cliente_nombre"`
	ClienteTelefono string  `json:"cliente_telefono"`
	Status          string  `json:"status"`
	FechaIngreso    string  `json:"fecha_ingreso"`
	Total           float64 `json:"total"`
	DiasEnStatus    int     `json:"dias_en_status"`
}

type ReparacionHistorial struct {
	ID            int    `json:"id"`
	StatusAnterior string `json:"status_anterior"`
	StatusNuevo   string `json:"status_nuevo"`
	UsuarioID     int    `json:"usuario_id"`
	UsuarioNombre string `json:"usuario_nombre"`
	Notas         string `json:"notas"`
	CreadoEn      string `json:"creado_en"`
}

type ReparacionPieza struct {
	ID              int     `json:"id"`
	ReparacionID    int     `json:"reparacion_id"`
	PiezaID         int     `json:"pieza_id"`
	PiezaNombre     string  `json:"pieza_nombre"`
	PiezaCodigo     string  `json:"pieza_codigo"`
	Cantidad        int     `json:"cantidad"`
	PrecioUnitario  float64 `json:"precio_unitario"`
	Subtotal        float64 `json:"subtotal"`
}

type ReparacionArchivo struct {
	ID          int    `json:"id"`
	ReparacionID int   `json:"reparacion_id"`
	Nombre      string `json:"nombre"`
	URL         string `json:"url"`
	Tipo        string `json:"tipo"`
	SubidoPor   int    `json:"subido_por"`
	CreadoEn    string `json:"creado_en"`
}

type ReparacionStats struct {
	HoyRecibidos     int `json:"hoy_recibidos"`
	EnProceso        int `json:"en_proceso"`
	EsperaAprobacion int `json:"espera_aprobacion"`
	ListosEntrega    int `json:"listos_entrega"`
	Atrasados        int `json:"atrasados"`
	TotalActivos     int `json:"total_activos"`
	CompletadosMes   int `json:"completados_mes"`
}
