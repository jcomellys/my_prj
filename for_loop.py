# Ejemplo de ciclo for con zip() - Productos y Precios

# Lista de productos
productos = ["Manzana", "Pan", "Leche", "Huevos", "Arroz"]

# Lista de precios correspondientes
precios = [25.50, 18.00, 32.00, 45.00, 28.50]

# Crear diccionario usando zip y for
catalogo = {}
for producto, precio in zip(productos, precios):
    catalogo[producto] = precio

# Mostrar el catalogo completo
print("=== CATALOGO DE PRODUCTOS ===")
for producto, precio in catalogo.items():
    print(f"{producto}: ${precio:.2f}")

# Buscar precio de un producto
print("\n=== CONSULTA DE PRECIO ===")
producto_buscar = input("Ingrese el nombre del producto: ")

if producto_buscar in catalogo:
    print(f"El precio de {producto_buscar} es: ${catalogo[producto_buscar]:.2f}")
else:
    print(f"Producto '{producto_buscar}' no encontrado")
