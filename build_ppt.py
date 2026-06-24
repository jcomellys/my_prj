# -*- coding: utf-8 -*-
"""Genera una presentacion institucional profesional sobre Magnetismo."""
from pptx import Presentation
from pptx.util import Inches, Pt, Emu
from pptx.dml.color import RGBColor
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from pptx.enum.shapes import MSO_SHAPE
from pptx.oxml.ns import qn

# ----------------------------------------------------------------------------
# Paleta institucional (inspirada en el material original)
# ----------------------------------------------------------------------------
NAVY      = RGBColor(0x1F, 0x2A, 0x44)   # azul profundo
TEAL      = RGBColor(0x12, 0x6E, 0x82)   # verde azulado
ORANGE    = RGBColor(0xE8, 0x7A, 0x22)   # naranja acento
ORANGE_LT = RGBColor(0xF5, 0xA6, 0x4B)
LIGHT     = RGBColor(0xF4, 0xF6, 0xF8)   # gris muy claro
GREY      = RGBColor(0x5B, 0x66, 0x70)
DARK      = RGBColor(0x23, 0x2A, 0x33)
WHITE     = RGBColor(0xFF, 0xFF, 0xFF)
CARD      = RGBColor(0xFF, 0xFF, 0xFF)
CARD_EDGE = RGBColor(0xE2, 0xE7, 0xEC)

FONT = "Calibri"
FONT_H = "Calibri"

prs = Presentation()
prs.slide_width  = Inches(13.333)
prs.slide_height = Inches(7.5)
SW, SH = prs.slide_width, prs.slide_height
BLANK = prs.slide_layouts[6]


# ----------------------------------------------------------------------------
# Helpers
# ----------------------------------------------------------------------------
def slide():
    return prs.slides.add_slide(BLANK)


def rect(s, x, y, w, h, fill, line=None, line_w=None, shape=MSO_SHAPE.RECTANGLE,
         shadow=False):
    sp = s.shapes.add_shape(shape, x, y, w, h)
    sp.fill.solid()
    sp.fill.fore_color.rgb = fill
    if line is None:
        sp.line.fill.background()
    else:
        sp.line.color.rgb = line
        sp.line.width = line_w or Pt(1)
    sp.shadow.inherit = False
    if shadow:
        el = sp._element.spPr
        ef = el.makeelement(qn('a:effectLst'), {})
        sh = el.makeelement(qn('a:outerShdw'),
                            {'blurRad': '90000', 'dist': '40000',
                             'dir': '5400000', 'rotWithShape': '0'})
        clr = el.makeelement(qn('a:srgbClr'), {'val': '1F2A44'})
        alpha = el.makeelement(qn('a:alpha'), {'val': '22000'})
        clr.append(alpha)
        sh.append(clr)
        ef.append(sh)
        el.append(ef)
    return sp


def txt(s, x, y, w, h, runs, align=PP_ALIGN.LEFT, anchor=MSO_ANCHOR.TOP,
        space_after=6, line_spacing=1.0):
    """runs: list of paragraphs; each paragraph = list of (text, size, color, bold, italic)."""
    tb = s.shapes.add_textbox(x, y, w, h)
    tf = tb.text_frame
    tf.word_wrap = True
    tf.vertical_anchor = anchor
    tf.margin_left = 0
    tf.margin_right = 0
    tf.margin_top = 0
    tf.margin_bottom = 0
    for i, para in enumerate(runs):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.alignment = align
        p.space_after = Pt(space_after)
        p.space_before = Pt(0)
        p.line_spacing = line_spacing
        for (t, sz, col, b, it) in para:
            r = p.add_run()
            r.text = t
            r.font.size = Pt(sz)
            r.font.color.rgb = col
            r.font.bold = b
            r.font.italic = it
            r.font.name = FONT
    return tb


def page_footer(s, n):
    txt(s, Inches(0.55), Inches(7.02), Inches(8), Inches(0.35),
        [[("Fundamentos de Transformadores  |  Principios y Aplicaciones", 9, GREY, False, False)]])
    txt(s, Inches(11.4), Inches(7.02), Inches(1.4), Inches(0.35),
        [[(f"{n:02d}", 9, ORANGE, True, False)]], align=PP_ALIGN.RIGHT)


def content_header(s, kicker, title, n):
    rect(s, 0, 0, SW, Inches(1.32), WHITE)
    rect(s, Inches(0.55), Inches(0.42), Inches(0.12), Inches(0.62), ORANGE)
    txt(s, Inches(0.82), Inches(0.34), Inches(11), Inches(0.32),
        [[(kicker.upper(), 11, TEAL, True, False)]])
    txt(s, Inches(0.82), Inches(0.62), Inches(11.8), Inches(0.6),
        [[(title, 26, NAVY, True, False)]])
    rect(s, 0, Inches(1.32), SW, Pt(2.2), LIGHT)
    rect(s, 0, Inches(1.32), Inches(3.2), Pt(2.2), ORANGE)
    page_footer(s, n)


def bullet(tf, first, text_runs, indent=0, bullet_color=ORANGE, space=8):
    p = tf.paragraphs[0] if first else tf.add_paragraph()
    p.space_after = Pt(space)
    p.space_before = Pt(0)
    p.line_spacing = 1.04
    p.level = indent
    # bullet glyph
    r0 = p.add_run()
    r0.text = "■  " if indent == 0 else "–  "
    r0.font.size = Pt(12 if indent == 0 else 12)
    r0.font.color.rgb = bullet_color if indent == 0 else ORANGE_LT
    r0.font.bold = True
    r0.font.name = FONT
    for (t, sz, col, b, it) in text_runs:
        r = p.add_run()
        r.text = t
        r.font.size = Pt(sz)
        r.font.color.rgb = col
        r.font.bold = b
        r.font.italic = it
        r.font.name = FONT


# ============================================================================
# SLIDE 1 — Portada
# ============================================================================
s = slide()
rect(s, 0, 0, SW, SH, NAVY)
# banda diagonal de acento
rect(s, 0, 0, Inches(4.7), SH, TEAL)
rect(s, 0, 0, Inches(0.28), SH, ORANGE)
# decorativo: lineas de campo (elipses concentricas a la derecha)
for i, r in enumerate([3.2, 2.55, 1.9, 1.25]):
    e = rect(s, Inches(10.4 - r/2), Inches(3.75 - r/2), Inches(r), Inches(r),
             NAVY, line=ORANGE_LT, line_w=Pt(1.1), shape=MSO_SHAPE.OVAL)
    e.fill.background()
rect(s, Inches(10.27), Inches(2.05), Inches(0.26), Inches(3.4), ORANGE,
     shape=MSO_SHAPE.ROUNDED_RECTANGLE)

txt(s, Inches(0.8), Inches(0.7), Inches(3.4), Inches(0.4),
    [[("PRESENTACION INSTITUCIONAL", 11, WHITE, True, False)]])
txt(s, Inches(0.8), Inches(1.0), Inches(3.4), Inches(0.4),
    [[("Capacitacion Tecnica", 11, ORANGE_LT, False, True)]])

txt(s, Inches(0.8), Inches(2.55), Inches(8.2), Inches(2.0),
    [[("Magnetismo", 54, WHITE, True, False)],
     [("y Fundamentos del Magnetismo", 30, ORANGE_LT, True, False)]],
    space_after=4)
txt(s, Inches(0.84), Inches(4.55), Inches(8.0), Inches(0.9),
    [[("Principios fisicos aplicados a motores, generadores y transformadores",
       15, RGBColor(0xC9, 0xD3, 0xDE), False, False)]])
rect(s, Inches(0.86), Inches(5.35), Inches(2.4), Pt(2.5), ORANGE)
txt(s, Inches(0.84), Inches(6.5), Inches(9), Inches(0.5),
    [[("Modulo 1  ·  Transformadores: Principios y Aplicaciones", 12,
       RGBColor(0xAF, 0xBC, 0xCB), False, False)]])

# ============================================================================
# SLIDE 2 — Agenda
# ============================================================================
s = slide()
rect(s, 0, 0, SW, SH, LIGHT)
content_header(s, "Contenido", "Agenda de la sesion", 2)
items = [
    ("01", "Que es el magnetismo", "Definicion y relevancia industrial"),
    ("02", "Magnetismo en la practica", "Generadores y motores electricos"),
    ("03", "Los imanes y el campo magnetico", "Materiales magneticos"),
    ("04", "Imanes permanentes y temporales", "Tipos y aplicaciones"),
    ("05", "Flujo magnetico y Faraday", "Lineas de fuerza y densidad de flujo"),
    ("06", "Glosario tecnico", "Definiciones clave del modulo"),
]
gx, gy = Inches(0.7), Inches(1.75)
cw, ch = Inches(5.95), Inches(1.45)
gapx, gapy = Inches(0.35), Inches(0.28)
for i, (num, t, d) in enumerate(items):
    col = i % 2
    row = i // 2
    x = gx + col * (cw + gapx)
    y = gy + row * (ch + gapy)
    rect(s, x, y, cw, ch, CARD, line=CARD_EDGE, line_w=Pt(1), shadow=True,
         shape=MSO_SHAPE.ROUNDED_RECTANGLE)
    rect(s, x, y, Inches(0.14), ch, ORANGE)
    txt(s, x + Inches(0.34), y + Inches(0.18), Inches(1.2), Inches(1),
        [[(num, 34, TEAL, True, False)]], anchor=MSO_ANCHOR.MIDDLE)
    txt(s, x + Inches(1.45), y + Inches(0.26), cw - Inches(1.6), Inches(1),
        [[(t, 17, NAVY, True, False)],
         [(d, 12, GREY, False, False)]], space_after=2)

# ============================================================================
# SLIDE 3 — Que es el magnetismo
# ============================================================================
s = slide()
rect(s, 0, 0, SW, SH, WHITE)
content_header(s, "01 · Concepto", "Que es el magnetismo", 3)
# panel destacado izquierda
rect(s, Inches(0.7), Inches(1.7), Inches(5.55), Inches(4.9), NAVY, shadow=True,
     shape=MSO_SHAPE.ROUNDED_RECTANGLE)
rect(s, Inches(0.7), Inches(1.7), Inches(5.55), Inches(0.16), ORANGE,
     shape=MSO_SHAPE.ROUNDED_RECTANGLE)
txt(s, Inches(1.05), Inches(2.15), Inches(4.9), Inches(0.5),
    [[("DEFINICION", 13, ORANGE_LT, True, False)]])
txt(s, Inches(1.05), Inches(2.65), Inches(4.95), Inches(2.4),
    [[("El magnetismo es una fuerza invisible mediante la cual los materiales se atraen o se repelen entre si.",
       21, WHITE, True, False)]], line_spacing=1.06)
rect(s, Inches(1.07), Inches(4.5), Inches(1.5), Pt(2.2), ORANGE)
txt(s, Inches(1.05), Inches(4.8), Inches(4.95), Inches(1.6),
    [[("Es la base para producir la mayor parte de la electricidad que consumimos y para generar movimiento en maquinas electricas.",
       13.5, RGBColor(0xC9, 0xD3, 0xDE), False, False)]], line_spacing=1.12)

# columna derecha: aplicaciones
txt(s, Inches(6.6), Inches(1.75), Inches(6.1), Inches(0.5),
    [[("El magnetismo permite:", 16, TEAL, True, False)]])
apps = [
    ("Generar electricidad", "Produce la mayor parte de la energia electrica que se consume."),
    ("Movimiento rotativo", "Desarrolla el giro en los motores electricos."),
    ("Movimiento lineal", "Genera desplazamiento lineal en los solenoides."),
]
ay = Inches(2.35)
for t, d in apps:
    rect(s, Inches(6.6), ay, Inches(6.05), Inches(1.18), LIGHT, line=CARD_EDGE,
         line_w=Pt(1), shape=MSO_SHAPE.ROUNDED_RECTANGLE)
    rect(s, Inches(6.6), ay, Inches(0.12), Inches(1.18), TEAL)
    txt(s, Inches(6.95), ay + Inches(0.16), Inches(5.5), Inches(0.45),
        [[(t, 15.5, NAVY, True, False)]])
    txt(s, Inches(6.95), ay + Inches(0.56), Inches(5.55), Inches(0.6),
        [[(d, 12.5, GREY, False, False)]], line_spacing=1.05)
    ay += Inches(1.4)

# ============================================================================
# SLIDE 4 — Generadores y motores
# ============================================================================
s = slide()
rect(s, 0, 0, SW, SH, LIGHT)
content_header(s, "02 · Aplicacion", "Generadores y motores electricos", 4)
txt(s, Inches(0.7), Inches(1.62), Inches(12), Inches(0.6),
    [[("Los generadores y los motores electricos operan bajo los mismos principios basicos del magnetismo.",
       15, GREY, False, True)]])

cards = [
    ("GENERADOR", "Convierte movimiento en electricidad", TEAL,
     "Utiliza el magnetismo para producir energia electrica a partir del movimiento mecanico."),
    ("MOTOR ELECTRICO", "Convierte electricidad en movimiento", ORANGE,
     "Emplea la energia electrica para desarrollar electromagnetismo en su interior, que hace girar el eje del motor."),
]
cx = Inches(0.7)
for (head, sub, col, body) in cards:
    rect(s, cx, Inches(2.4), Inches(5.95), Inches(3.0), CARD, line=CARD_EDGE,
         line_w=Pt(1), shadow=True, shape=MSO_SHAPE.ROUNDED_RECTANGLE)
    rect(s, cx, Inches(2.4), Inches(5.95), Inches(0.85), col,
         shape=MSO_SHAPE.ROUND_2_SAME_RECTANGLE)
    txt(s, cx + Inches(0.4), Inches(2.5), Inches(5.2), Inches(0.65),
        [[(head, 20, WHITE, True, False)]], anchor=MSO_ANCHOR.MIDDLE)
    txt(s, cx + Inches(0.4), Inches(3.45), Inches(5.2), Inches(0.5),
        [[(sub, 14.5, col, True, False)]])
    txt(s, cx + Inches(0.4), Inches(4.0), Inches(5.2), Inches(1.3),
        [[(body, 14, DARK, False, False)]], line_spacing=1.14)
    cx += Inches(6.4)

# barra inferior idea-fuerza
rect(s, Inches(0.7), Inches(5.75), Inches(11.95), Inches(0.95), NAVY,
     shape=MSO_SHAPE.ROUNDED_RECTANGLE)
rect(s, Inches(0.7), Inches(5.75), Inches(0.14), Inches(0.95), ORANGE)
txt(s, Inches(1.1), Inches(5.75), Inches(11.3), Inches(0.95),
    [[("La intensidad de las fuerzas magneticas determina el torque que el eje del motor puede entregar.",
       14.5, WHITE, False, True)]], anchor=MSO_ANCHOR.MIDDLE)

# ============================================================================
# SLIDE 5 — Imanes y campo magnetico
# ============================================================================
s = slide()
rect(s, 0, 0, SW, SH, WHITE)
content_header(s, "03 · Materiales", "Los imanes y el campo magnetico", 5)

rect(s, Inches(0.7), Inches(1.7), Inches(5.6), Inches(2.05), TEAL, shadow=True,
     shape=MSO_SHAPE.ROUNDED_RECTANGLE)
txt(s, Inches(1.05), Inches(1.95), Inches(4.95), Inches(0.4),
    [[("IMAN", 13, WHITE, True, False)]])
txt(s, Inches(1.05), Inches(2.3), Inches(4.95), Inches(1.3),
    [[("Material que produce un campo magnetico.", 19, WHITE, True, False)]],
    line_spacing=1.05)

rect(s, Inches(0.7), Inches(3.95), Inches(5.6), Inches(2.05), NAVY, shadow=True,
     shape=MSO_SHAPE.ROUNDED_RECTANGLE)
txt(s, Inches(1.05), Inches(4.2), Inches(4.95), Inches(0.4),
    [[("CAMPO MAGNETICO", 13, ORANGE_LT, True, False)]])
txt(s, Inches(1.05), Inches(4.55), Inches(4.95), Inches(1.3),
    [[("Ejerce una fuerza sobre cargas electricas en movimiento o sobre otros imanes.",
       17, WHITE, True, False)]], line_spacing=1.05)

# derecha: materiales magneticos
txt(s, Inches(6.7), Inches(1.75), Inches(6), Inches(0.5),
    [[("Materiales magneticos", 17, TEAL, True, False)]])
txt(s, Inches(6.7), Inches(2.2), Inches(6), Inches(0.6),
    [[("La mayoria de los imanes se fabrican con aleaciones de distintos materiales magneticos.",
       13, GREY, False, False)]], line_spacing=1.1)

rect(s, Inches(6.7), Inches(3.0), Inches(5.95), Inches(1.5), LIGHT,
     line=CARD_EDGE, line_w=Pt(1), shape=MSO_SHAPE.ROUNDED_RECTANGLE)
rect(s, Inches(6.7), Inches(3.0), Inches(0.12), Inches(1.5), TEAL)
txt(s, Inches(7.0), Inches(3.18), Inches(5.5), Inches(0.4),
    [[("Comunes (naturales)", 13, NAVY, True, False)]])
txt(s, Inches(7.0), Inches(3.6), Inches(5.5), Inches(0.8),
    [[("Hierro  ·  Niquel  ·  Cobalto", 18, TEAL, True, False)]])

rect(s, Inches(6.7), Inches(4.7), Inches(5.95), Inches(1.5), LIGHT,
     line=CARD_EDGE, line_w=Pt(1), shape=MSO_SHAPE.ROUNDED_RECTANGLE)
rect(s, Inches(6.7), Inches(4.7), Inches(0.12), Inches(1.5), ORANGE)
txt(s, Inches(7.0), Inches(4.88), Inches(5.5), Inches(0.4),
    [[("Menos comunes (tierras raras)", 13, NAVY, True, False)]])
txt(s, Inches(7.0), Inches(5.3), Inches(5.5), Inches(0.8),
    [[("Neodimio  ·  Samario  ·  otros", 18, ORANGE, True, False)]])

# ============================================================================
# SLIDE 6 — Imanes permanentes y temporales
# ============================================================================
s = slide()
rect(s, 0, 0, SW, SH, LIGHT)
content_header(s, "04 · Clasificacion", "Imanes permanentes y temporales", 6)

# Permanente
px = Inches(0.7)
rect(s, px, Inches(1.7), Inches(5.95), Inches(4.9), CARD, line=CARD_EDGE,
     line_w=Pt(1), shadow=True, shape=MSO_SHAPE.ROUNDED_RECTANGLE)
rect(s, px, Inches(1.7), Inches(5.95), Inches(0.95), TEAL,
     shape=MSO_SHAPE.ROUND_2_SAME_RECTANGLE)
txt(s, px + Inches(0.4), Inches(1.78), Inches(5.2), Inches(0.8),
    [[("IMAN PERMANENTE", 19, WHITE, True, False)]], anchor=MSO_ANCHOR.MIDDLE)
tb = s.shapes.add_textbox(px + Inches(0.42), Inches(2.85), Inches(5.15), Inches(3.6))
tf = tb.text_frame; tf.word_wrap = True
bullet(tf, True, [("Conserva su magnetismo durante un periodo prolongado.", 14, DARK, False, False)], bullet_color=TEAL)
bullet(tf, False, [("Tipos: ", 14, DARK, True, False), ("naturales (magnetita) y fabricados.", 14, DARK, False, False)], bullet_color=TEAL)
bullet(tf, False, [("Aplicaciones: ", 14, DARK, True, False), ("motores CC de iman permanente e interruptores de lengueta (reed).", 14, DARK, False, False)], bullet_color=TEAL)
bullet(tf, False, [("Ejemplos comunes: ", 14, DARK, True, False), ("imanes de herradura, brujulas e imanes de barra.", 14, DARK, False, False)], bullet_color=TEAL)

# Temporal
tx = Inches(6.65 + 0.0)
tx = Inches(6.7 + 0.0)
tx = Inches(6.7) + Inches(0.0)
tx = Inches(7.08)
rect(s, tx, Inches(1.7), Inches(5.95), Inches(4.9), CARD, line=CARD_EDGE,
     line_w=Pt(1), shadow=True, shape=MSO_SHAPE.ROUNDED_RECTANGLE)
rect(s, tx, Inches(1.7), Inches(5.95), Inches(0.95), ORANGE,
     shape=MSO_SHAPE.ROUND_2_SAME_RECTANGLE)
txt(s, tx + Inches(0.4), Inches(1.78), Inches(5.2), Inches(0.8),
    [[("IMAN TEMPORAL", 19, WHITE, True, False)]], anchor=MSO_ANCHOR.MIDDLE)
tb = s.shapes.add_textbox(tx + Inches(0.42), Inches(2.85), Inches(5.15), Inches(3.6))
tf = tb.text_frame; tf.word_wrap = True
bullet(tf, True, [("Retiene solo trazas de magnetismo al retirar la fuerza magnetizante.", 14, DARK, False, False)])
bullet(tf, False, [("Retentividad: ", 14, DARK, True, False), ("mide la capacidad del iman de conservar magnetismo.", 14, DARK, False, False)])
bullet(tf, False, [("Aplicaciones: ", 14, DARK, True, False), ("motores, transformadores y solenoides.", 14, DARK, False, False)])
bullet(tf, False, [("Mas comunes: ", 14, DARK, True, False), ("bobinas conductoras usadas como electroimanes.", 14, DARK, False, False)])

# ============================================================================
# SLIDE 7 — Flujo magnetico y Faraday
# ============================================================================
s = slide()
rect(s, 0, 0, SW, SH, WHITE)
content_header(s, "05 · Flujo magnetico", "Flujo magnetico y Michael Faraday", 7)

# tarjeta Faraday
rect(s, Inches(0.7), Inches(1.7), Inches(4.0), Inches(4.9), NAVY, shadow=True,
     shape=MSO_SHAPE.ROUNDED_RECTANGLE)
rect(s, Inches(0.7), Inches(1.7), Inches(4.0), Inches(0.16), ORANGE,
     shape=MSO_SHAPE.ROUNDED_RECTANGLE)
# avatar circulo
av = rect(s, Inches(1.9), Inches(2.15), Inches(1.6), Inches(1.6), TEAL,
          shape=MSO_SHAPE.OVAL)
txt(s, Inches(1.9), Inches(2.15), Inches(1.6), Inches(1.6),
    [[("MF", 40, WHITE, True, False)]], align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
txt(s, Inches(0.9), Inches(3.95), Inches(3.6), Inches(0.5),
    [[("Michael Faraday", 19, WHITE, True, False)]], align=PP_ALIGN.CENTER)
txt(s, Inches(0.9), Inches(4.35), Inches(3.6), Inches(0.4),
    [[("Cientifico ingles", 12.5, ORANGE_LT, False, True)]], align=PP_ALIGN.CENTER)
txt(s, Inches(1.0), Inches(4.95), Inches(3.4), Inches(1.5),
    [[("Sus descubrimientos incluyeron la induccion electromagnetica y la rotacion electromagnetica. Propuso el concepto de campo magnetico para visualizarlo.",
       12.5, RGBColor(0xC9, 0xD3, 0xDE), False, False)]], line_spacing=1.12,
    align=PP_ALIGN.CENTER)

# conceptos derecha
rect(s, Inches(4.95), Inches(1.7), Inches(7.7), Inches(2.35), LIGHT,
     line=CARD_EDGE, line_w=Pt(1), shape=MSO_SHAPE.ROUNDED_RECTANGLE)
rect(s, Inches(4.95), Inches(1.7), Inches(0.14), Inches(2.35), TEAL)
txt(s, Inches(5.3), Inches(1.95), Inches(7.1), Inches(0.5),
    [[("Flujo magnetico", 18, TEAL, True, False)]])
txt(s, Inches(5.3), Inches(2.45), Inches(7.1), Inches(1.4),
    [[("Tambien llamado ", 14, DARK, False, False),
      ("flujo de campo", 14, DARK, True, True),
      (". Es la cantidad total de un campo magnetico, representada como ", 14, DARK, False, False),
      ("lineas de fuerza", 14, NAVY, True, False), (".", 14, DARK, False, False)]],
    line_spacing=1.15)

rect(s, Inches(4.95), Inches(4.25), Inches(7.7), Inches(2.35), LIGHT,
     line=CARD_EDGE, line_w=Pt(1), shape=MSO_SHAPE.ROUNDED_RECTANGLE)
rect(s, Inches(4.95), Inches(4.25), Inches(0.14), Inches(2.35), ORANGE)
txt(s, Inches(5.3), Inches(4.5), Inches(7.1), Inches(0.5),
    [[("Densidad de flujo", 18, ORANGE, True, False)]])
txt(s, Inches(5.3), Inches(5.0), Inches(7.1), Inches(1.5),
    [[("Es la densidad del flujo magnetico en un area especifica. A mayor concentracion, ", 14, DARK, False, False),
      ("mayor fuerza magnetica", 14, NAVY, True, False),
      (". El flujo es mas denso en los extremos: la fuerza es maxima en los ", 14, DARK, False, False),
      ("polos del iman", 14, NAVY, True, False), (".", 14, DARK, False, False)]],
    line_spacing=1.15)

# ============================================================================
# SLIDE 8 — Glosario
# ============================================================================
s = slide()
rect(s, 0, 0, SW, SH, LIGHT)
content_header(s, "06 · Referencia", "Glosario tecnico", 8)
terms = [
    ("Magnetismo", "Fuerza invisible por la que los materiales se atraen o se repelen."),
    ("Iman", "Material que produce un campo magnetico."),
    ("Campo magnetico", "Ejerce fuerza sobre cargas en movimiento u otros imanes."),
    ("Iman permanente", "Conserva su magnetismo durante un periodo prolongado."),
    ("Iman temporal", "Retiene solo trazas de magnetismo al retirar la fuerza magnetizante."),
    ("Retentividad", "Capacidad de un iman de conservar el magnetismo."),
    ("Flujo magnetico", "Cantidad total del campo, mostrada como lineas de fuerza."),
    ("Densidad de flujo", "Densidad del flujo magnetico en un area especifica."),
]
gx, gy = Inches(0.7), Inches(1.7)
cw, ch = Inches(5.95), Inches(1.18)
gapx, gapy = Inches(0.35), Inches(0.18)
for i, (t, d) in enumerate(terms):
    col = i % 2
    row = i // 2
    x = gx + col * (cw + gapx)
    y = gy + row * (ch + gapy)
    rect(s, x, y, cw, ch, CARD, line=CARD_EDGE, line_w=Pt(1),
         shape=MSO_SHAPE.ROUNDED_RECTANGLE)
    rect(s, x, y, Inches(0.12), ch, TEAL if col == 0 else ORANGE)
    txt(s, x + Inches(0.34), y + Inches(0.13), cw - Inches(0.5), Inches(0.45),
        [[(t, 14.5, NAVY, True, False)]])
    txt(s, x + Inches(0.34), y + Inches(0.52), cw - Inches(0.5), Inches(0.6),
        [[(d, 11.5, GREY, False, False)]], line_spacing=1.05)

# ============================================================================
# SLIDE 9 — Cierre
# ============================================================================
s = slide()
rect(s, 0, 0, SW, SH, NAVY)
rect(s, 0, 0, SW, Inches(0.28), ORANGE)
for i, r in enumerate([3.0, 2.35, 1.7]):
    e = rect(s, Inches(2.4 - r/2), Inches(3.75 - r/2), Inches(r), Inches(r),
             NAVY, line=TEAL, line_w=Pt(1.1), shape=MSO_SHAPE.OVAL)
    e.fill.background()
txt(s, Inches(4.0), Inches(2.5), Inches(8.6), Inches(1.2),
    [[("Ideas clave", 40, WHITE, True, False)]])
rect(s, Inches(4.04), Inches(3.45), Inches(2.0), Pt(2.5), ORANGE)
pts = [
    "El magnetismo es la base de la generacion de energia y del movimiento en maquinas electricas.",
    "Los imanes pueden ser permanentes o temporales segun su retentividad.",
    "El flujo y su densidad explican por que la fuerza es maxima en los polos.",
]
yy = Inches(3.85)
for p in pts:
    rect(s, Inches(4.04), yy + Inches(0.08), Inches(0.18), Inches(0.18), ORANGE,
         shape=MSO_SHAPE.OVAL)
    txt(s, Inches(4.4), yy, Inches(8.2), Inches(0.8),
        [[(p, 15, RGBColor(0xD7, 0xDE, 0xE8), False, False)]], line_spacing=1.1)
    yy += Inches(0.92)
txt(s, Inches(4.0), Inches(6.75), Inches(8), Inches(0.4),
    [[("Gracias  ·  Fundamentos de Transformadores", 12, RGBColor(0xAF, 0xBC, 0xCB), False, False)]])

out = "/home/user/my_prj/Magnetismo_Presentacion_Institucional.pptx"
prs.save(out)
print("Guardado:", out, "| Slides:", len(prs.slides._sldIdLst))
