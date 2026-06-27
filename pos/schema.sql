-- Esquema para Refacciones de Celular
-- Tablas principales: marcas, modelos, categorias, piezas, compatibilidad

CREATE TABLE IF NOT EXISTS marcas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    nombre TEXT NOT NULL UNIQUE,
    logo_url TEXT,
    activa INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now','localtime'))
);

CREATE TABLE IF NOT EXISTS modelos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    marca_id INTEGER NOT NULL REFERENCES marcas(id),
    nombre TEXT NOT NULL,
    año_lanzamiento INTEGER,
    imagen_url TEXT,
    activo INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now','localtime')),
    UNIQUE(marca_id, nombre)
);

CREATE TABLE IF NOT EXISTS categorias (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    nombre TEXT NOT NULL UNIQUE,
    slug TEXT NOT NULL UNIQUE,
    icono TEXT,
    descripcion TEXT,
    orden INTEGER DEFAULT 0,
    activa INTEGER DEFAULT 1
);

CREATE TABLE IF NOT EXISTS piezas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    codigo TEXT NOT NULL UNIQUE,
    categoria_id INTEGER NOT NULL REFERENCES categorias(id),
    nombre TEXT NOT NULL,
    descripcion TEXT,
    precio REAL NOT NULL DEFAULT 0,
    costo REAL NOT NULL DEFAULT 0,
    stock INTEGER NOT NULL DEFAULT 0,
    stock_minimo INTEGER DEFAULT 5,
    imagen_url TEXT,
    imagenes_adicionales TEXT, -- JSON array de URLs
    estado TEXT DEFAULT 'nuevo', -- nuevo, reacondicionado, usado
    garantia_dias INTEGER DEFAULT 90,
    proveedor TEXT,
    ubicacion TEXT, -- estante/bodega
    activa INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now','localtime')),
    updated_at TEXT DEFAULT (datetime('now','localtime'))
);

-- Compatibilidad: qué piezas sirven para qué modelos
CREATE TABLE IF NOT EXISTS compatibilidad (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pieza_id INTEGER NOT NULL REFERENCES piezas(id) ON DELETE CASCADE,
    modelo_id INTEGER NOT NULL REFERENCES modelos(id) ON DELETE CASCADE,
    notas TEXT, -- ej: "solo versión 64GB", "requiere actualización iOS"
    UNIQUE(pieza_id, modelo_id)
);

-- Usuarios (solo admin y vendedores, no helpers)
CREATE TABLE IF NOT EXISTS usuarios (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    nombre_completo TEXT,
    direccion TEXT,
    telefono TEXT,
    usuario TEXT NOT NULL UNIQUE,
    clave TEXT NOT NULL,
    activo TEXT DEFAULT 't',
    rol TEXT DEFAULT 'vendedor', -- admin, vendedor
    created_at TEXT DEFAULT (datetime('now','localtime')),
    correo TEXT,
    foto TEXT
);

-- Sesiones
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES usuarios(id),
    created_at TEXT DEFAULT (datetime('now','localtime')),
    expires_at TEXT NOT NULL
);

-- Ventas simplificadas (solo para admin/vendedor)
CREATE TABLE IF NOT EXISTS ventas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    folio INTEGER NOT NULL,
    cliente_nombre TEXT,
    cliente_telefono TEXT,
    cliente_email TEXT,
    total REAL NOT NULL,
    forma_pago TEXT DEFAULT 'efectivo', -- efectivo, tarjeta, transferencia
    vendedor_id INTEGER REFERENCES usuarios(id),
    creado_en TEXT DEFAULT (datetime('now','localtime')),
    notas TEXT
);

CREATE TABLE IF NOT EXISTS ventas_detalle (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    venta_id INTEGER NOT NULL REFERENCES ventas(id) ON DELETE CASCADE,
    pieza_id INTEGER NOT NULL REFERENCES piezas(id),
    cantidad INTEGER NOT NULL DEFAULT 1,
    precio_unitario REAL NOT NULL,
    subtotal REAL NOT NULL
);

-- Índices para búsqueda rápida
CREATE INDEX IF NOT EXISTS idx_piezas_categoria ON piezas(categoria_id);
CREATE INDEX IF NOT EXISTS idx_piezas_codigo ON piezas(codigo);
CREATE INDEX IF NOT EXISTS idx_piezas_nombre ON piezas(nombre);
CREATE INDEX IF NOT EXISTS idx_piezas_activa ON piezas(activa);
CREATE INDEX IF NOT EXISTS idx_compatibilidad_pieza ON compatibilidad(pieza_id);
CREATE INDEX IF NOT EXISTS idx_compatibilidad_modelo ON compatibilidad(modelo_id);
CREATE INDEX IF NOT EXISTS idx_modelos_marca ON modelos(marca_id);
CREATE INDEX IF NOT EXISTS idx_modelos_activo ON modelos(activo);

-- Configuración del sistema
CREATE TABLE IF NOT EXISTS config (
    clave TEXT PRIMARY KEY,
    valor TEXT NOT NULL DEFAULT ''
);

-- Datos iniciales: config
INSERT OR IGNORE INTO config (clave, valor) VALUES ('negocio_nombre', 'RefacCel'), ('tema', 'light'), ('telefono', '');

-- Datos iniciales: categorías
INSERT OR IGNORE INTO categorias (nombre, slug, icono, descripcion, orden) VALUES
('Pantallas', 'pantallas', 'smartphone', 'Pantallas completas y displays', 1),
('Baterías', 'baterias', 'battery', 'Baterías internas y externas', 2),
('Cámaras', 'camaras', 'camera', 'Cámaras traseras y frontales', 3),
('Placas Base', 'placas', 'cpu', 'Motherboards y placas lógicas', 4),
('Flex / Conectores', 'flex', 'plug', 'Flex cables, conectores de carga, jack', 5),
('Carcasas / Chasis', 'carcasas', 'case', 'Tapas traseras, marcos, carcasas completas', 6),
('Botones / Switches', 'botones', 'toggle', 'Botones de encendido, volumen, home', 7),
('Altavoces / Micrófonos', 'audio', 'speaker', 'Speakers, micrófonos, auriculares', 8),
('Sensores', 'sensores', 'sensor', 'Huella, FaceID, proximidad, giroscopio', 9),
('Herramientas', 'herramientas', 'wrench', 'Kits de apertura, ventosas, pinzas', 10);

-- Marcas principales (celulares + laptops/PC + audio)
INSERT OR IGNORE INTO marcas (nombre) VALUES
('Apple'), ('Samsung'), ('Xiaomi'), ('Motorola'), ('Huawei'), ('Oppo'), ('Vivo'), ('Realme'), ('OnePlus'), ('Google'), ('Nokia'), ('Sony'), ('LG'), ('Alcatel'), ('TCL'), ('ZTE'), ('Dell'), ('HP'), ('Lenovo'), ('ASUS'), ('Acer'), ('Microsoft'), ('Toshiba'), ('Panasonic'), ('Harman Kardon'), ('JBL'), ('Bose'), ('Sennheiser'), ('AMD'), ('Intel'), ('NVIDIA'), ('Kingston'), ('Corsair'), ('Seagate'), ('Western Digital'), ('Crucial'), ('Otras');

-- ─── Reparaciones ────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS reparaciones (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    folio INTEGER NOT NULL,
    token TEXT NOT NULL UNIQUE DEFAULT (lower(hex(randomblob(16)))),
    tipo_equipo TEXT NOT NULL DEFAULT 'laptop', -- laptop, pc, celular, audio, otro
    marca_id INTEGER REFERENCES marcas(id),
    modelo_texto TEXT,
    numero_serie TEXT,
    imei TEXT,
    password_equipo TEXT,
    condicion_fisica TEXT,
    cliente_nombre TEXT NOT NULL,
    cliente_telefono TEXT,
    cliente_email TEXT,
    falla_reportada TEXT NOT NULL,
    diagnostico TEXT,
    accesorios TEXT,
    notas_cliente TEXT,
    notas_internas TEXT,
    status TEXT NOT NULL DEFAULT 'recibido',
    fecha_ingreso TEXT DEFAULT (datetime('now','localtime')),
    fecha_prometida TEXT,
    fecha_entrega TEXT,
    costo_diagnostico REAL DEFAULT 0,
    costo_reparacion REAL DEFAULT 0,
    total REAL DEFAULT 0,
    anticipo REAL DEFAULT 0,
    tecnico_id INTEGER REFERENCES usuarios(id),
    activo INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now','localtime')),
    updated_at TEXT DEFAULT (datetime('now','localtime'))
);

CREATE TABLE IF NOT EXISTS reparaciones_piezas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reparacion_id INTEGER NOT NULL REFERENCES reparaciones(id) ON DELETE CASCADE,
    pieza_id INTEGER NOT NULL REFERENCES piezas(id),
    cantidad INTEGER NOT NULL DEFAULT 1,
    precio_unitario REAL NOT NULL,
    subtotal REAL NOT NULL
);

CREATE TABLE IF NOT EXISTS reparaciones_archivos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reparacion_id INTEGER NOT NULL REFERENCES reparaciones(id) ON DELETE CASCADE,
    nombre TEXT NOT NULL,
    url TEXT NOT NULL,
    tipo TEXT DEFAULT 'foto', -- foto, pdf, otro
    subido_por INTEGER REFERENCES usuarios(id),
    created_at TEXT DEFAULT (datetime('now','localtime'))
);

CREATE TABLE IF NOT EXISTS reparaciones_historial (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reparacion_id INTEGER NOT NULL REFERENCES reparaciones(id) ON DELETE CASCADE,
    status_anterior TEXT,
    status_nuevo TEXT NOT NULL,
    usuario_id INTEGER REFERENCES usuarios(id),
    notas TEXT,
    creado_en TEXT DEFAULT (datetime('now','localtime'))
);

CREATE INDEX IF NOT EXISTS idx_reparaciones_status ON reparaciones(status);
CREATE INDEX IF NOT EXISTS idx_reparaciones_token ON reparaciones(token);
CREATE INDEX IF NOT EXISTS idx_reparaciones_folio ON reparaciones(folio);
CREATE INDEX IF NOT EXISTS idx_reparaciones_cliente ON reparaciones(cliente_nombre);
CREATE INDEX IF NOT EXISTS idx_reparaciones_marca ON reparaciones(marca_id);
CREATE INDEX IF NOT EXISTS idx_reparaciones_piezas_rep ON reparaciones_piezas(reparacion_id);
CREATE INDEX IF NOT EXISTS idx_reparaciones_historial_rep ON reparaciones_historial(reparacion_id);
CREATE INDEX IF NOT EXISTS idx_reparaciones_archivos_rep ON reparaciones_archivos(reparacion_id);

CREATE TRIGGER IF NOT EXISTS update_reparaciones_timestamp
AFTER UPDATE ON reparaciones
BEGIN
    UPDATE reparaciones SET updated_at = datetime('now','localtime') WHERE id = NEW.id;
END;

-- Trigger: auto-log historial on status change
CREATE TRIGGER IF NOT EXISTS log_reparacion_status
AFTER UPDATE OF status ON reparaciones
BEGIN
    INSERT INTO reparaciones_historial (reparacion_id, status_anterior, status_nuevo)
    VALUES (NEW.id, OLD.status, NEW.status);
END;

-- Trigger: auto-decrement stock when repair part is added
CREATE TRIGGER IF NOT EXISTS reparacion_descontar_stock
AFTER INSERT ON reparaciones_piezas
BEGIN
    UPDATE piezas SET stock = stock - NEW.cantidad WHERE id = NEW.pieza_id;
END;

-- Trigger: restore stock when repair part is removed
CREATE TRIGGER IF NOT EXISTS reparacion_restaurar_stock
AFTER DELETE ON reparaciones_piezas
BEGIN
    UPDATE piezas SET stock = stock + OLD.cantidad WHERE id = OLD.pieza_id;
END;

-- Trigger para updated_at en piezas
CREATE TRIGGER IF NOT EXISTS update_piezas_timestamp
AFTER UPDATE ON piezas
BEGIN
    UPDATE piezas SET updated_at = datetime('now','localtime') WHERE id = NEW.id;
END;