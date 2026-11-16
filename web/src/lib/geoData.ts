// Ported from the Personal CRM desktop app (src/geo.rs). Rows are "name|alias…;CC;lat;lon";
// countries are "CC|name|alias…;lat;lon". Loaded lazily by lib/geo.ts.

export const COUNTRIES = `
AF|Afghanistan;33.9;67.7
AL|Albania;41.2;20.2
DZ|Algeria;28.0;2.6
AD|Andorra;42.5;1.5
AO|Angola;-11.2;17.9
AR|Argentina;-38.4;-63.6
AM|Armenia;40.1;45.0
AU|Australia;-25.3;133.8
AT|Austria|osterreich;47.5;14.6
AZ|Azerbaijan;40.1;47.6
BS|Bahamas|the bahamas;25.0;-77.4
BH|Bahrain;26.0;50.6
BD|Bangladesh;23.7;90.4
BB|Barbados;13.2;-59.5
BY|Belarus;53.7;28.0
BE|Belgium|belgique|belgie;50.5;4.5
BZ|Belize;17.2;-88.5
BJ|Benin;9.3;2.3
BT|Bhutan;27.5;90.4
BO|Bolivia;-16.3;-63.6
BA|Bosnia and Herzegovina|bosnia;43.9;17.7
BW|Botswana;-22.3;24.7
BR|Brazil|brasil;-14.2;-51.9
BN|Brunei;4.5;114.7
BG|Bulgaria;42.7;25.5
BF|Burkina Faso;12.2;-1.6
BI|Burundi;-3.4;29.9
KH|Cambodia;12.6;104.9
CM|Cameroon;7.4;12.4
CA|Canada;56.1;-106.3
CV|Cape Verde|cabo verde;16.0;-24.0
CF|Central African Republic;6.6;20.9
TD|Chad;15.5;18.7
CL|Chile;-35.7;-71.5
CN|China|people's republic of china;35.9;104.2
CO|Colombia;4.6;-74.3
KM|Comoros;-11.9;43.9
CG|Congo|republic of the congo;-0.2;15.8
CD|Democratic Republic of the Congo|dr congo|drc;-4.0;21.8
CR|Costa Rica;9.7;-83.8
CI|Cote d'Ivoire|ivory coast;7.5;-5.5
HR|Croatia|hrvatska;45.1;15.2
CU|Cuba;21.5;-77.8
CY|Cyprus;35.1;33.4
CZ|Czechia|czech republic;49.8;15.5
DK|Denmark|danmark;56.3;9.5
DJ|Djibouti;11.8;42.6
DO|Dominican Republic;18.7;-70.2
EC|Ecuador;-1.8;-78.2
EG|Egypt;26.8;30.8
SV|El Salvador;13.8;-88.9
GQ|Equatorial Guinea;1.7;10.3
ER|Eritrea;15.2;39.8
EE|Estonia|eesti;58.6;25.0
SZ|Eswatini|swaziland;-26.5;31.5
ET|Ethiopia;9.1;40.5
FJ|Fiji;-17.7;178.1
FI|Finland|suomi;61.9;25.7
FR|France;46.6;2.2
GA|Gabon;-0.8;11.6
GM|Gambia|the gambia;13.4;-15.3
GE|Georgia;42.3;43.4
DE|Germany|deutschland;51.2;10.4
GH|Ghana;7.9;-1.0
GR|Greece|hellas;39.1;21.8
GL|Greenland;71.7;-42.6
GT|Guatemala;15.8;-90.2
GN|Guinea;9.9;-9.7
GW|Guinea-Bissau;11.8;-15.2
GY|Guyana;4.9;-58.9
HT|Haiti;19.0;-72.3
HN|Honduras;15.2;-86.2
HK|Hong Kong|hong kong sar|hong kong sar china;22.3;114.2
HU|Hungary|magyarorszag;47.2;19.5
IS|Iceland;64.9;-19.0
IN|India;20.6;79.0
ID|Indonesia;-0.8;113.9
IR|Iran;32.4;53.7
IQ|Iraq;33.2;43.7
IE|Ireland|republic of ireland|eire;53.4;-8.2
IL|Israel;31.0;34.9
IT|Italy|italia;41.9;12.6
JM|Jamaica;18.1;-77.3
JP|Japan;36.2;138.3
JO|Jordan;30.6;36.2
KZ|Kazakhstan;48.0;66.9
KE|Kenya;-0.0;37.9
XK|Kosovo;42.6;20.9
KW|Kuwait;29.3;47.5
KG|Kyrgyzstan;41.2;74.8
LA|Laos;19.9;102.5
LV|Latvia;56.9;24.6
LB|Lebanon;33.9;35.9
LS|Lesotho;-29.6;28.2
LR|Liberia;6.4;-9.4
LY|Libya;26.3;17.2
LI|Liechtenstein;47.2;9.6
LT|Lithuania|lietuva;55.2;23.9
LU|Luxembourg;49.8;6.1
MO|Macao|macau|macao sar;22.2;113.5
MG|Madagascar;-18.8;46.9
MW|Malawi;-13.3;34.3
MY|Malaysia;4.2;102.0
MV|Maldives;3.2;73.2
ML|Mali;17.6;-4.0
MT|Malta;35.9;14.4
MR|Mauritania;21.0;-10.9
MU|Mauritius;-20.3;57.6
MX|Mexico;23.6;-102.6
MD|Moldova;47.4;28.4
MC|Monaco;43.7;7.4
MN|Mongolia;46.9;103.8
ME|Montenegro;42.7;19.4
MA|Morocco;31.8;-7.1
MZ|Mozambique;-18.7;35.5
MM|Myanmar|burma;21.9;95.9
NA|Namibia;-22.9;18.5
NP|Nepal;28.4;84.1
NL|Netherlands|the netherlands|nederland|holland;52.1;5.3
NZ|New Zealand|aotearoa;-40.9;174.9
NI|Nicaragua;12.9;-85.2
NE|Niger;17.6;8.1
NG|Nigeria;9.1;8.7
KP|North Korea;40.3;127.5
MK|North Macedonia|macedonia;41.6;21.7
NO|Norway|norge;60.5;8.5
OM|Oman;21.5;55.9
PK|Pakistan;30.4;69.3
PS|Palestine|palestinian territories;31.9;35.2
PA|Panama;8.5;-80.8
PG|Papua New Guinea;-6.3;143.9
PY|Paraguay;-23.4;-58.4
PE|Peru;-9.2;-75.0
PH|Philippines;12.9;121.8
PL|Poland|polska;51.9;19.1
PT|Portugal;39.4;-8.2
PR|Puerto Rico;18.2;-66.6
QA|Qatar;25.4;51.2
RO|Romania;45.9;25.0
RU|Russia|russian federation;61.5;105.3
RW|Rwanda;-1.9;29.9
SA|Saudi Arabia;23.9;45.1
SN|Senegal;14.5;-14.5
RS|Serbia|srbija;44.0;21.0
SC|Seychelles;-4.7;55.5
SL|Sierra Leone;8.5;-11.8
SG|Singapore;1.35;103.8
SK|Slovakia;48.7;19.7
SI|Slovenia;46.2;15.0
SO|Somalia;5.2;46.2
ZA|South Africa;-30.6;22.9
KR|South Korea|korea|republic of korea;35.9;127.8
SS|South Sudan;6.9;31.3
ES|Spain|espana;40.5;-3.7
LK|Sri Lanka;7.9;80.8
SD|Sudan;12.9;30.2
SR|Suriname;3.9;-56.0
SE|Sweden|sverige;60.1;18.6
CH|Switzerland|schweiz|suisse|svizzera;46.8;8.2
SY|Syria;34.8;39.0
TW|Taiwan;23.7;121.0
TJ|Tajikistan;38.9;71.3
TZ|Tanzania;-6.4;34.9
TH|Thailand;15.9;100.99
TG|Togo;8.6;0.8
TT|Trinidad and Tobago;10.7;-61.2
TN|Tunisia;33.9;9.5
TR|Turkey|turkiye;39.0;35.2
TM|Turkmenistan;38.97;59.6
UG|Uganda;1.4;32.3
UA|Ukraine;48.4;31.2
AE|United Arab Emirates|uae;23.4;53.8
GB|United Kingdom|uk|great britain|britain;54.0;-2.5
US|United States|usa|united states of america|us;39.8;-98.6
UY|Uruguay;-32.5;-55.8
UZ|Uzbekistan;41.4;64.6
VE|Venezuela;6.4;-66.6
VN|Vietnam|viet nam;14.1;108.3
YE|Yemen;15.6;48.5
ZM|Zambia;-13.1;27.8
ZW|Zimbabwe;-19.0;29.2
`

export const REGIONS = `
England;GB;52.5;-1.5
Scotland;GB;56.5;-4.2
Wales;GB;52.3;-3.7
Northern Ireland;GB;54.6;-6.7
Oxfordshire;GB;51.8;-1.3
Buckinghamshire;GB;51.8;-0.8
Berkshire;GB;51.45;-1.1
Surrey;GB;51.25;-0.4
Kent;GB;51.2;0.7
Essex;GB;51.8;0.6
Sussex|east sussex|west sussex;GB;50.9;-0.3
Hampshire;GB;51.05;-1.3
Hertfordshire;GB;51.8;-0.2
Yorkshire|north yorkshire|west yorkshire|south yorkshire;GB;53.95;-1.3
Lancashire;GB;53.8;-2.6
Cornwall;GB;50.4;-4.9
Devon;GB;50.7;-3.8
Gloucestershire;GB;51.85;-2.2
Wiltshire;GB;51.3;-1.9
Somerset;GB;51.1;-3.0
Cambridgeshire;GB;52.3;0.05
Norfolk;GB;52.6;1.0
Suffolk;GB;52.2;1.0
Cheshire;GB;53.2;-2.5
Northumberland;GB;55.2;-2.0
County Durham|durham county;GB;54.7;-1.7
Île-de-France|ile de france;FR;48.85;2.35
Grand Est;FR;48.7;5.9
Hauts-de-France;FR;50.0;2.8
Normandy|normandie;FR;49.2;0.0
Brittany|bretagne;FR;48.2;-2.9
Pays de la Loire;FR;47.5;-0.8
Centre-Val de Loire;FR;47.5;1.7
Bourgogne-Franche-Comté;FR;47.2;4.8
Auvergne-Rhône-Alpes;FR;45.5;4.5
Nouvelle-Aquitaine;FR;45.2;0.2
Occitanie;FR;43.7;2.2
Provence-Alpes-Côte d'Azur|paca;FR;43.9;6.1
Corsica|corse;FR;42.1;9.0
Baden-Württemberg|baden wurttemberg;DE;48.6;9.0
Bavaria|bayern;DE;48.9;11.4
Berlin;DE;52.52;13.4
Brandenburg;DE;52.4;13.0
Bremen;DE;53.08;8.8
Hamburg;DE;53.55;10.0
Hesse|hessen;DE;50.65;9.0
Lower Saxony|niedersachsen;DE;52.6;9.8
Mecklenburg-Vorpommern;DE;53.6;12.4
North Rhine-Westphalia|nordrhein westfalen|nrw;DE;51.4;7.6
Rhineland-Palatinate|rheinland pfalz;DE;49.9;7.4
Saarland;DE;49.4;6.95
Saxony|sachsen;DE;51.05;13.4
Saxony-Anhalt|sachsen anhalt;DE;51.95;11.7
Schleswig-Holstein;DE;54.2;9.7
Thuringia|thuringen;DE;50.9;11.0
Lombardy|lombardia;IT;45.6;9.8
Latium|lazio;IT;41.9;12.7
Piedmont|piemonte;IT;45.05;7.9
Veneto;IT;45.6;11.9
Tuscany|toscana;IT;43.4;11.0
Emilia-Romagna;IT;44.5;11.0
Campania;IT;40.9;14.6
Sicily|sicilia;IT;37.5;14.1
Sardinia|sardegna;IT;40.1;9.0
Apulia|puglia;IT;41.0;16.5
Liguria;IT;44.3;8.7
Community of Madrid|comunidad de madrid;ES;40.4;-3.7
Catalonia|cataluna|catalunya;ES;41.8;1.5
Andalusia|andalucia;ES;37.5;-4.7
Valencian Community|comunidad valenciana;ES;39.4;-0.6
Basque Country|pais vasco;ES;43.0;-2.6
Galicia;ES;42.75;-7.9
Balearic Islands|illes balears;ES;39.6;2.9
Canary Islands;ES;28.3;-15.8
Vaud;CH;46.6;6.6
Zurich|zürich;CH;47.4;8.65
Geneva|geneve|genève;CH;46.2;6.15
Bern;CH;46.8;7.6
Basel|basel stadt;CH;47.55;7.6
St Gallen|st. gallen|sankt gallen;CH;47.3;9.2
Schwyz;CH;47.05;8.75
Lucerne|luzern;CH;47.05;8.3
Ticino;CH;46.3;8.8
Zug;CH;47.15;8.5
Vienna|wien;AT;48.2;16.37
Tyrol|tirol;AT;47.2;11.4
Salzburg;AT;47.8;13.05
Upper Austria;AT;48.1;14.0
Styria|steiermark;AT;47.2;15.0
Brussels Region|brussels capital region;BE;50.85;4.35
Flanders;BE;51.0;4.3
Wallonia;BE;50.4;4.7
North Holland|noord holland;NL;52.6;4.85
South Holland|zuid holland;NL;52.0;4.5
Utrecht;NL;52.1;5.2
Capital Region of Denmark|hovedstaden;DK;55.7;12.4
Västra Götaland County|vastra gotaland;SE;58.0;12.5
Stockholm County;SE;59.3;18.1
Dolnośląskie|lower silesia;PL;51.1;16.5
Masovia|mazowieckie;PL;52.3;21.0
Timiş|timis;RO;45.75;21.2
Attiki|attica;GR;38.0;23.7
County Dublin;IE;53.35;-6.26
Federation of Bosnia and Herzegovina;BA;43.85;18.4
Rabat-Salé-Kénitra;MA;34.0;-6.8
Lagos State;NG;6.5;3.4
Federal District|distrito federal;BR;-15.8;-47.9
São Paulo|sao paulo state;BR;-22.0;-48.5
New South Wales;AU;-32.0;147.0
Victoria;AU;-37.0;144.5
Queensland;AU;-22.0;145.0
Western Australia;AU;-26.0;121.0
Ontario;CA;50.0;-85.0
Quebec|québec;CA;52.0;-72.0
British Columbia;CA;54.0;-125.0
Alberta;CA;55.0;-115.0
Nova Scotia;CA;45.0;-63.0
Alabama;US;32.8;-86.8
Alaska;US;64.0;-150.0
Arizona;US;34.3;-111.7
Arkansas;US;34.9;-92.4
California;US;37.2;-119.5
Colorado;US;39.0;-105.5
Connecticut;US;41.6;-72.7
Delaware;US;39.0;-75.5
District of Columbia|dc;US;38.9;-77.04
Florida;US;28.6;-82.4
Georgia State;US;32.7;-83.4
Hawaii;US;20.8;-156.3
Idaho;US;44.4;-114.6
Illinois;US;40.0;-89.2
Indiana;US;39.9;-86.3
Iowa;US;42.1;-93.5
Kansas;US;38.5;-98.4
Kentucky;US;37.5;-85.3
Louisiana;US;31.1;-92.0
Maine;US;45.4;-69.2
Maryland;US;39.0;-76.8
Massachusetts;US;42.3;-71.8
Michigan;US;44.3;-85.4
Minnesota;US;46.3;-94.3
Mississippi;US;32.7;-89.7
Missouri;US;38.4;-92.5
Montana;US;47.0;-109.6
Nebraska;US;41.5;-99.8
Nevada;US;39.3;-116.6
New Hampshire;US;43.7;-71.6
New Jersey;US;40.2;-74.7
New Mexico;US;34.4;-106.1
New York State;US;42.9;-75.5
North Carolina;US;35.6;-79.4
North Dakota;US;47.5;-100.5
Ohio;US;40.3;-82.8
Oklahoma;US;35.6;-97.5
Oregon;US;43.9;-120.6
Pennsylvania;US;40.9;-77.8
Rhode Island;US;41.7;-71.5
South Carolina;US;33.9;-80.9
South Dakota;US;44.4;-100.2
Tennessee;US;35.9;-86.4
Texas;US;31.5;-99.3
Utah;US;39.3;-111.7
Vermont;US;44.1;-72.7
Virginia;US;37.5;-78.9
Washington State;US;47.4;-120.5
West Virginia;US;38.6;-80.6
Wisconsin;US;44.6;-89.9
Wyoming;US;43.0;-107.5
`

export const CITIES = `
London|city of london|greater london|london area;GB;51.51;-0.13
Oxford;GB;51.75;-1.26
Cambridge;GB;52.2;0.12
Durham;GB;54.78;-1.57
Reading;GB;51.45;-0.97
Manchester;GB;53.48;-2.24
Birmingham;GB;52.49;-1.89
Liverpool;GB;53.41;-2.98
Leeds;GB;53.8;-1.55
Sheffield;GB;53.38;-1.47
Bristol;GB;51.45;-2.59
Newcastle upon Tyne|newcastle|tyneside;GB;54.98;-1.61
Sunderland;GB;54.91;-1.38
Nottingham;GB;52.95;-1.15
Leicester;GB;52.64;-1.13
Brighton;GB;50.82;-0.14
Southampton;GB;50.9;-1.4
Portsmouth;GB;50.8;-1.09
Edinburgh;GB;55.95;-3.19
Glasgow;GB;55.86;-4.25
Aberdeen;GB;57.15;-2.09
Inverness;GB;57.48;-4.22
St Andrews;GB;56.34;-2.8
Cardiff;GB;51.48;-3.18
Swansea;GB;51.62;-3.94
Monmouth;GB;51.81;-2.72
Belfast;GB;54.6;-5.93
Bath;GB;51.38;-2.36
Exeter;GB;50.72;-3.53
York;GB;53.96;-1.08
Lancaster;GB;54.05;-2.8
Winchester;GB;51.06;-1.31
Henley on Thames|henley;GB;51.54;-0.9
Wantage;GB;51.59;-1.43
Wallingford;GB;51.6;-1.12
Benson;GB;51.62;-1.11
Warborough;GB;51.64;-1.13
Chipping Norton;GB;51.94;-1.55
Lechlade on Thames|lechlade;GB;51.69;-1.69
Weston on the Green;GB;51.86;-1.23
Abingdon;GB;51.67;-1.28
Banbury;GB;52.06;-1.34
Bicester;GB;51.9;-1.15
Witney;GB;51.78;-1.49
Thame;GB;51.75;-0.98
Aylesbury;GB;51.82;-0.81
Amersham;GB;51.67;-0.61
Beaconsfield;GB;51.61;-0.64
Gerrards Cross;GB;51.59;-0.55
Chalfont St Peter;GB;51.61;-0.56
Great Missenden;GB;51.7;-0.71
Buckingham;GB;52.0;-0.99
Marlow;GB;51.57;-0.77
High Wycombe;GB;51.63;-0.75
Farnham Royal;GB;51.54;-0.62
Windsor;GB;51.48;-0.61
Slough;GB;51.51;-0.59
Maidenhead;GB;51.52;-0.72
Ascot;GB;51.41;-0.68
Eton;GB;51.49;-0.61
Esher;GB;51.37;-0.37
Weybridge;GB;51.37;-0.45
Chobham;GB;51.35;-0.6
Guildford;GB;51.24;-0.57
Woking;GB;51.32;-0.56
Oxted;GB;51.26;-0.0
Richmond upon Thames|richmond;GB;51.46;-0.3
Teddington;GB;51.42;-0.33
Kingston upon Thames;GB;51.41;-0.3
Wimbledon;GB;51.42;-0.21
Swanscombe;GB;51.45;0.31
Marlborough;GB;51.42;-1.73
Ramsbury;GB;51.44;-1.6
Burghclere;GB;51.34;-1.33
Fawley;GB;50.82;-1.35
Newbury;GB;51.4;-1.32
Sherborne;GB;50.95;-2.52
Whitchurch;GB;51.23;-1.34
Whichford;GB;52.01;-1.56
Northampton;GB;52.24;-0.9
Hinckley;GB;52.54;-1.37
Darlington;GB;54.52;-1.55
Yarm;GB;54.51;-1.36
Wolsingham;GB;54.73;-1.88
Alnwick;GB;55.41;-1.71
Birkenhead;GB;53.39;-3.02
Brandeston;GB;52.2;1.29
Norwich;GB;52.63;1.3
Ipswich;GB;52.06;1.16
Canterbury;GB;51.28;1.08
Milton Keynes;GB;52.04;-0.76
Coventry;GB;52.41;-1.51
Warwick;GB;52.28;-1.58
Cheltenham;GB;51.9;-2.08
Gloucester;GB;51.86;-2.24
Plymouth;GB;50.38;-4.14
Bournemouth;GB;50.72;-1.88
Dublin;IE;53.35;-6.26
Cork;IE;51.9;-8.47
Galway;IE;53.27;-9.05
Paris|greater paris|paris metropolitan region|ile de france paris;FR;48.86;2.35
Reims;FR;49.26;4.03
Lyon;FR;45.76;4.84
Marseille;FR;43.3;5.37
Toulouse;FR;43.6;1.44
Nice;FR;43.7;7.27
Menton;FR;43.78;7.5
Nantes;FR;47.22;-1.55
Strasbourg;FR;48.57;7.75
Montpellier;FR;43.61;3.88
Bordeaux;FR;44.84;-0.58
Lille;FR;50.63;3.06
Rennes;FR;48.11;-1.68
Le Havre;FR;49.49;0.11
Rouen;FR;49.44;1.1
Caen;FR;49.18;-0.37
Nancy;FR;48.69;6.18
Metz;FR;49.12;6.18
Dijon;FR;47.32;5.04
Grenoble;FR;45.19;5.72
Clermont Ferrand;FR;45.78;3.08
Chambery;FR;45.56;5.92
Annecy;FR;45.9;6.13
Vichy;FR;46.13;3.43
Dax;FR;43.71;-1.05
Allauch;FR;43.34;5.48
Alfortville;FR;48.8;2.42
Ermont;FR;48.99;2.26
Versailles;FR;48.8;2.13
Boulogne Billancourt;FR;48.84;2.24
Neuilly sur Seine;FR;48.88;2.27
Saint Denis;FR;48.94;2.36
Cannes;FR;43.55;7.01
Antibes;FR;43.58;7.12
Aix en Provence;FR;43.53;5.45
Avignon;FR;43.95;4.81
Tours;FR;47.39;0.69
Angers;FR;47.47;-0.55
Orleans;FR;47.9;1.9
Poitiers;FR;46.58;0.34
Limoges;FR;45.83;1.26
Brest;FR;48.39;-4.49
Le Mans;FR;48.0;0.2
Amiens;FR;49.89;2.3
Troyes;FR;48.3;4.08
Epernay;FR;49.04;3.96
Chalons en Champagne;FR;48.96;4.36
Besancon;FR;47.24;6.02
Mulhouse;FR;47.75;7.34
Colmar;FR;48.08;7.36
Perpignan;FR;42.7;2.9
Pau;FR;43.3;-0.37
Biarritz;FR;43.48;-1.56
Bayonne;FR;43.49;-1.47
La Rochelle;FR;46.16;-1.15
Theix Noyalo|theix;FR;47.63;-2.66
Vannes;FR;47.66;-2.76
Monaco|monte carlo;MC;43.74;7.42
Berlin|berlin metropolitan area;DE;52.52;13.4
Munich|munchen|münchen|greater munich;DE;48.14;11.58
Hamburg|greater hamburg;DE;53.55;10.0
Frankfurt|frankfurt am main|frankfurt rhine main|frankfurt rhine main metropolitan;DE;50.11;8.68
Cologne|koln|köln;DE;50.94;6.96
Dusseldorf|düsseldorf;DE;51.23;6.78
Stuttgart;DE;48.78;9.18
Dortmund;DE;51.51;7.47
Essen;DE;51.46;7.01
Leipzig;DE;51.34;12.37
Dresden;DE;51.05;13.74
Hanover|hannover;DE;52.37;9.73
Nuremberg|nurnberg|nürnberg;DE;49.45;11.08
Bremen;DE;53.08;8.8
Bonn;DE;50.73;7.1
Mannheim;DE;49.49;8.47
Karlsruhe;DE;49.01;8.4
Heidelberg;DE;49.4;8.69
Freiburg|freiburg im breisgau;DE;47.99;7.84
Constance|konstanz;DE;47.66;9.18
Radolfzell;DE;47.74;8.97
Salem;DE;47.76;9.28
Mainz;DE;50.0;8.27
Wiesbaden;DE;50.08;8.24
Bad Homburg|bad homburg vor der hohe;DE;50.23;8.62
Regensburg;DE;49.01;12.1
Augsburg;DE;48.37;10.9
Wurzburg|würzburg;DE;49.79;9.95
Ulm;DE;48.4;9.99
Tubingen|tübingen;DE;48.52;9.06
Heilbronn;DE;49.14;9.22
Ludwigsburg;DE;48.9;9.19
Esslingen;DE;48.74;9.31
Backnang;DE;48.95;9.43
Allmersbach im Tal;DE;48.91;9.47
Heddesheim;DE;49.5;8.6
Kassel|greater kassel;DE;51.31;9.48
Brunswick|braunschweig;DE;52.27;10.52
Wolfenbuttel|wolfenbüttel;DE;52.16;10.53
Neustadt am Rubenberge|neustadt am rübenberge;DE;52.5;9.46
Gottingen|göttingen;DE;51.54;9.92
Munster|münster;DE;51.96;7.63
Aachen;DE;50.78;6.08
Kiel;DE;54.32;10.12
Lubeck|lübeck;DE;53.87;10.69
Rostock;DE;54.09;12.1
Potsdam;DE;52.39;13.06
Erfurt;DE;50.98;11.03
Jena;DE;50.93;11.59
Magdeburg;DE;52.13;11.63
Saarbrucken|saarbrücken;DE;49.24;6.99
Trier;DE;49.75;6.64
Koblenz;DE;50.36;7.59
Darmstadt;DE;49.87;8.65
Passau;DE;48.57;13.43
Bamberg;DE;49.89;10.89
Bayreuth;DE;49.95;11.58
Ingolstadt;DE;48.76;11.42
Vienna|wien;AT;48.21;16.37
Salzburg;AT;47.81;13.04
Innsbruck;AT;47.27;11.39
Graz;AT;47.07;15.44
Linz;AT;48.31;14.29
Zurich|zürich;CH;47.38;8.54
Geneva|geneve|genève;CH;46.2;6.14
Basel;CH;47.56;7.59
Bern|berne;CH;46.95;7.45
Lausanne;CH;46.52;6.63
Lucerne|luzern|lucerne metropolitan;CH;47.05;8.31
St Gallen|st. gallen|sankt gallen;CH;47.42;9.37
Zug;CH;47.17;8.52
Lugano;CH;46.0;8.95
Wollerau;CH;47.19;8.72
Grabs;CH;47.18;9.44
Vaduz;LI;47.14;9.52
Brussels|bruxelles|brussel|brussels metropolitan;BE;50.85;4.35
Antwerp|antwerpen;BE;51.22;4.4
Ghent|gent;BE;51.05;3.72
Leuven;BE;50.88;4.7
Liege|liège;BE;50.63;5.57
Bruges|brugge;BE;51.21;3.22
Luxembourg|luxembourg city;LU;49.61;6.13
Amsterdam;NL;52.37;4.9
Rotterdam;NL;51.92;4.48
The Hague|den haag;NL;52.08;4.3
Utrecht;NL;52.09;5.12
Eindhoven;NL;51.44;5.47
Leiden;NL;52.16;4.49
Groningen;NL;53.22;6.57
Maastricht;NL;50.85;5.69
Enschede|greater enschede;NL;52.22;6.89
Copenhagen|kobenhavn|københavn|copenhagen metropolitan;DK;55.68;12.57
Aarhus;DK;56.16;10.2
Stockholm;SE;59.33;18.07
Gothenburg|goteborg|göteborg;SE;57.71;11.97
Malmo|malmö;SE;55.6;13.0
Uppsala;SE;59.86;17.64
Lund;SE;55.7;13.19
Oslo|greater oslo;NO;59.91;10.75
Bergen;NO;60.39;5.32
Trondheim;NO;63.43;10.4
Helsinki;FI;60.17;24.94
Espoo;FI;60.21;24.66
Tampere;FI;61.5;23.76
Reykjavik|reykjavík;IS;64.15;-21.94
Tallinn;EE;59.44;24.75
Riga;LV;56.95;24.11
Vilnius|vilniaus;LT;54.69;25.28
Kaunas;LT;54.9;23.9
Warsaw|warszawa|warsaw metropolitan;PL;52.23;21.01
Krakow|kraków;PL;50.06;19.94
Wroclaw|wrocław;PL;51.11;17.03
Gdansk|gdańsk;PL;54.35;18.65
Poznan|poznań;PL;52.41;16.93
Lodz|łódź;PL;51.76;19.46
Prague|praha;CZ;50.08;14.44
Brno;CZ;49.2;16.61
Bratislava;SK;48.15;17.11
Budapest;HU;47.5;19.04
Debrecen;HU;47.53;21.63
Ljubljana;SI;46.06;14.51
Zagreb;HR;45.81;15.98
Split;HR;43.51;16.44
Belgrade|beograd;RS;44.79;20.45
Novi Sad;RS;45.27;19.83
Sarajevo|center sarajevo;BA;43.86;18.41
Podgorica;ME;42.44;19.26
Tirana;AL;41.33;19.82
Skopje;MK;41.99;21.43
Pristina|prishtina;XK;42.66;21.17
Sofia;BG;42.7;23.32
Plovdiv;BG;42.14;24.75
Bucharest|bucuresti;RO;44.43;26.1
Cluj Napoca|cluj;RO;46.77;23.6
Timisoara|timişoara|timișoara;RO;45.76;21.23
Iasi|iași;RO;47.16;27.59
Chisinau|chișinău;MD;47.01;28.86
Kyiv|kiev;UA;50.45;30.52
Lviv;UA;49.84;24.03
Odesa|odessa;UA;46.48;30.72
Kharkiv;UA;49.99;36.23
Minsk;BY;53.9;27.56
Moscow|moskva;RU;55.76;37.62
Saint Petersburg|st petersburg;RU;59.93;30.36
Novosibirsk;RU;55.01;82.93
Yekaterinburg;RU;56.84;60.6
Vladivostok;RU;43.12;131.89
Athens|athina;GR;37.98;23.73
Thessaloniki;GR;40.64;22.94
Nicosia;CY;35.17;33.36
Limassol;CY;34.68;33.04
Valletta;MT;35.9;14.51
Istanbul;TR;41.01;28.98
Ankara;TR;39.93;32.86
Izmir;TR;38.42;27.14
Antalya;TR;36.9;30.7
Tbilisi;GE;41.72;44.79
Batumi;GE;41.64;41.64
Yerevan;AM;40.18;44.51
Baku;AZ;40.41;49.87
Madrid;ES;40.42;-3.7
Barcelona;ES;41.39;2.17
Valencia;ES;39.47;-0.38
Seville|sevilla;ES;37.39;-5.98
Malaga|málaga;ES;36.72;-4.42
Bilbao;ES;43.26;-2.93
San Sebastian|donostia;ES;43.32;-1.98
Zaragoza;ES;41.65;-0.88
Palma|palma de mallorca;ES;39.57;2.65
Granada;ES;37.18;-3.6
Salamanca;ES;40.97;-5.66
Lisbon|lisboa;PT;38.72;-9.14
Porto;PT;41.15;-8.61
Coimbra;PT;40.21;-8.43
Faro;PT;37.02;-7.93
Rome|roma;IT;41.9;12.5
Milan|milano;IT;45.46;9.19
Turin|torino;IT;45.07;7.69
Naples|napoli;IT;40.85;14.27
Florence|firenze;IT;43.77;11.26
Bologna;IT;44.49;11.34
Venice|venezia;IT;45.44;12.32
Genoa|genova;IT;44.41;8.93
Palermo;IT;38.12;13.36
Verona;IT;45.44;10.99
Padua|padova;IT;45.41;11.88
Pisa;IT;43.72;10.4
Trieste;IT;45.65;13.78
Bari;IT;41.12;16.87
Catania;IT;37.5;15.09
Bergamo;IT;45.7;9.67
Como;IT;45.81;9.09
New York|new york city|nyc|manhattan|new york city metropolitan;US;40.71;-74.01
Brooklyn;US;40.68;-73.94
Queens;US;40.73;-73.79
Bronx|the bronx;US;40.84;-73.86
Mamaroneck;US;40.95;-73.73
Stony Brook;US;40.91;-73.12
Crown Point;US;43.95;-73.43
White Plains;US;41.03;-73.76
Yonkers;US;40.93;-73.9
Buffalo;US;42.89;-78.88
Rochester;US;43.16;-77.61
Ithaca;US;42.44;-76.5
Albany;US;42.65;-73.75
Jersey City;US;40.73;-74.07
Newark;US;40.74;-74.17
Princeton;US;40.36;-74.66
Hoboken;US;40.74;-74.03
Philadelphia;US;39.95;-75.17
Pittsburgh;US;40.44;-79.99
Boston;US;42.36;-71.06
Cambridge MA;US;42.37;-71.11
New Haven;US;41.31;-72.93
Hartford;US;41.76;-72.67
Providence;US;41.82;-71.41
Washington|washington dc|washington d.c.|washington dc baltimore;US;38.91;-77.04
Baltimore;US;39.29;-76.61
Bethesda;US;38.98;-77.1
Arlington;US;38.88;-77.1
Alexandria;US;38.8;-77.05
Richmond VA;US;37.54;-77.44
Charlotte;US;35.23;-80.84
Raleigh;US;35.78;-78.64
Durham NC;US;35.99;-78.9
Chapel Hill;US;35.91;-79.06
Atlanta;US;33.75;-84.39
Miami;US;25.76;-80.19
Orlando;US;28.54;-81.38
Tampa;US;27.95;-82.46
Jacksonville;US;30.33;-81.66
Nashville;US;36.16;-86.78
Chicago;US;41.88;-87.63
Evanston;US;42.05;-87.69
Detroit;US;42.33;-83.05
Ann Arbor;US;42.28;-83.74
Cleveland;US;41.5;-81.69
Columbus;US;39.96;-83.0
Cincinnati;US;39.1;-84.51
Indianapolis;US;39.77;-86.16
Notre Dame|south bend;US;41.7;-86.24
Milwaukee;US;43.04;-87.91
Madison;US;43.07;-89.4
Minneapolis;US;44.98;-93.27
St Paul|saint paul;US;44.95;-93.09
St Louis|saint louis;US;38.63;-90.2
Kansas City;US;39.1;-94.58
Omaha;US;41.26;-95.93
Denver;US;39.74;-104.99
Boulder;US;40.01;-105.27
Evergreen;US;39.63;-105.32
Salt Lake City;US;40.76;-111.89
Phoenix;US;33.45;-112.07
Las Vegas;US;36.17;-115.14
Albuquerque;US;35.08;-106.65
Dallas|dallas fort worth;US;32.78;-96.8
Fort Worth;US;32.76;-97.33
Houston;US;29.76;-95.37
Austin;US;30.27;-97.74
San Antonio;US;29.42;-98.49
El Paso;US;31.76;-106.49
New Orleans;US;29.95;-90.07
Los Angeles|la|los angeles metropolitan;US;34.05;-118.24
San Francisco|sf|san francisco bay|bay area;US;37.77;-122.42
San Jose;US;37.34;-121.89
Palo Alto;US;37.44;-122.14
Stanford;US;37.43;-122.17
Mountain View;US;37.39;-122.08
Sunnyvale;US;37.37;-122.04
Menlo Park;US;37.45;-122.18
Cupertino;US;37.32;-122.03
Santa Clara;US;37.35;-121.96
Redwood City;US;37.49;-122.24
Belmont;US;37.52;-122.28
Oakland;US;37.8;-122.27
Berkeley;US;37.87;-122.27
San Diego;US;32.72;-117.16
Irvine;US;33.68;-117.83
Santa Monica;US;34.02;-118.49
Pasadena;US;34.15;-118.14
Santa Barbara;US;34.42;-119.7
Sacramento;US;38.58;-121.49
Seattle;US;47.61;-122.33
Redmond;US;47.67;-122.12
Bellevue;US;47.61;-122.2
Portland;US;45.52;-122.68
Honolulu;US;21.31;-157.86
Anchorage;US;61.22;-149.9
Toronto|greater toronto;CA;43.65;-79.38
Montreal|montréal;CA;45.5;-73.57
Vancouver;CA;49.28;-123.12
Ottawa|greater ottawa;CA;45.42;-75.7
Calgary;CA;51.05;-114.07
Edmonton;CA;53.55;-113.49
Quebec City;CA;46.81;-71.21
Winnipeg;CA;49.9;-97.14
Halifax;CA;44.65;-63.58
Waterloo;CA;43.46;-80.52
Mexico City|ciudad de mexico|cdmx;MX;19.43;-99.13
Guadalajara;MX;20.66;-103.35
Monterrey;MX;25.69;-100.32
Cancun|cancún;MX;21.16;-86.85
Havana|la habana;CU;23.11;-82.37
San Juan;PR;18.47;-66.11
Panama City;PA;8.98;-79.52
San Jose CR;CR;9.93;-84.08
Bogota|bogotá;CO;4.71;-74.07
Medellin|medellín;CO;6.24;-75.58
Caracas;VE;10.48;-66.9
Quito;EC;-0.18;-78.47
Lima;PE;-12.05;-77.04
La Paz;BO;-16.5;-68.15
Santiago;CL;-33.45;-70.67
Buenos Aires;AR;-34.6;-58.38
Montevideo;UY;-34.9;-56.16
Asuncion|asunción;PY;-25.26;-57.58
Sao Paulo|são paulo;BR;-23.55;-46.63
Rio de Janeiro;BR;-22.91;-43.17
Brasilia|brasília;BR;-15.79;-47.88
Belo Horizonte;BR;-19.92;-43.94
Porto Alegre;BR;-30.03;-51.23
Curitiba;BR;-25.43;-49.27
Cairo;EG;30.04;31.24
Alexandria EG;EG;31.2;29.92
Casablanca;MA;33.57;-7.59
Rabat|rabat sale kenitra;MA;34.02;-6.84
Marrakesh|marrakech;MA;31.63;-8.0
Tunis;TN;36.81;10.18
Algiers;DZ;36.75;3.06
Lagos;NG;6.52;3.38
Abuja;NG;9.08;7.4
Accra;GH;5.6;-0.19
Dakar;SN;14.72;-17.47
Abidjan;CI;5.36;-4.01
Nairobi;KE;-1.29;36.82
Addis Ababa;ET;9.03;38.74
Kampala;UG;0.35;32.58
Kigali;RW;-1.95;30.06
Dar es Salaam;TZ;-6.79;39.21
Johannesburg;ZA;-26.2;28.05
Cape Town;ZA;-33.92;18.42
Durban;ZA;-29.86;31.02
Pretoria;ZA;-25.75;28.19
Luanda;AO;-8.84;13.23
Kinshasa;CD;-4.44;15.27
Tel Aviv|tel aviv yafo;IL;32.09;34.78
Jerusalem;IL;31.77;35.21
Beirut;LB;33.89;35.5
Amman;JO;31.95;35.93
Riyadh;SA;24.71;46.68
Jeddah;SA;21.49;39.19
Dubai;AE;25.2;55.27
Abu Dhabi;AE;24.45;54.38
Doha;QA;25.29;51.53
Manama;BH;26.23;50.59
Kuwait City;KW;29.38;47.99
Muscat;OM;23.59;58.41
Tehran;IR;35.69;51.39
Baghdad;IQ;33.31;44.36
Erbil;IQ;36.19;44.01
Karachi;PK;24.86;67.0
Lahore;PK;31.52;74.36
Islamabad;PK;33.68;73.05
Kabul;AF;34.56;69.21
Tashkent;UZ;41.3;69.24
Almaty;KZ;43.24;76.89
Astana|nur sultan;KZ;51.17;71.45
Bishkek;KG;42.87;74.59
Mumbai|bombay;IN;19.08;72.88
Delhi|new delhi;IN;28.61;77.21
Bangalore|bengaluru;IN;12.97;77.59
Hyderabad;IN;17.39;78.49
Chennai;IN;13.08;80.27
Kolkata;IN;22.57;88.36
Pune;IN;18.52;73.86
Gurgaon|gurugram;IN;28.46;77.03
Noida;IN;28.54;77.39
Ahmedabad;IN;23.02;72.57
Colombo;LK;6.93;79.86
Kathmandu;NP;27.72;85.32
Dhaka;BD;23.81;90.41
Bangkok;TH;13.76;100.5
Chiang Mai;TH;18.79;98.98
Hanoi;VN;21.03;105.85
Ho Chi Minh City|saigon;VN;10.82;106.63
Da Nang;VN;16.05;108.2
Phnom Penh;KH;11.56;104.93
Yangon;MM;16.87;96.2
Kuala Lumpur;MY;3.14;101.69
Penang|george town;MY;5.41;100.33
Singapore;SG;1.35;103.82
Jakarta;ID;-6.21;106.85
Bali|denpasar;ID;-8.65;115.22
Surabaya;ID;-7.25;112.75
Manila|metro manila;PH;14.6;120.98
Cebu|cebu city;PH;10.32;123.89
Hong Kong;HK;22.32;114.17
Macau|macao;MO;22.2;113.54
Shenzhen;CN;22.54;114.06
Guangzhou;CN;23.13;113.26
Shanghai;CN;31.23;121.47
Beijing;CN;39.9;116.41
Hangzhou;CN;30.27;120.16
Nanjing;CN;32.06;118.8
Chengdu;CN;30.57;104.07
Wuhan;CN;30.59;114.31
Xi'an|xian;CN;34.34;108.94
Tianjin;CN;39.34;117.36
Suzhou;CN;31.3;120.58
Taipei;TW;25.03;121.57
Hsinchu;TW;24.8;120.97
Seoul;KR;37.57;126.98
Busan;KR;35.18;129.08
Tokyo;JP;35.68;139.69
Nerima;JP;35.74;139.65
Yokohama;JP;35.44;139.64
Osaka;JP;34.69;135.5
Kyoto;JP;35.01;135.77
Nagoya;JP;35.18;136.91
Fukuoka;JP;33.59;130.4
Sapporo;JP;43.06;141.35
Ulaanbaatar;MN;47.89;106.91
Sydney;AU;-33.87;151.21
Manly;AU;-33.8;151.29
Melbourne;AU;-37.81;144.96
Brisbane;AU;-27.47;153.03
Perth;AU;-31.95;115.86
Adelaide;AU;-34.93;138.6
Canberra;AU;-35.28;149.13
Gold Coast;AU;-28.02;153.4
Hobart;AU;-42.88;147.33
Auckland;NZ;-36.85;174.76
Wellington;NZ;-41.29;174.78
Christchurch;NZ;-43.53;172.64
Queenstown;NZ;-45.03;168.66
`

/** Coarse coastlines as flat [lon, lat, lon, lat, …] rings, combined with the even-odd rule. */
export const LAND: number[][] = [
	// North America
	[
		-168.0, 66.0, -162.0, 70.0, -156.0, 71.3, -148.0, 70.3, -140.0, 69.6, -130.0, 70.0, -124.0, 69.5, -115.0, 68.8,
		-108.0, 68.5, -98.0, 68.0, -94.0, 71.0, -90.0, 68.5, -85.0, 69.5, -82.0, 66.0, -87.0, 64.0, -93.0, 61.0,
		-94.0, 58.5, -90.0, 57.0, -85.0, 55.2, -82.0, 52.5, -79.0, 51.5, -79.0, 54.5, -77.0, 58.0, -78.0, 62.3,
		-73.0, 62.0, -69.0, 59.0, -65.0, 60.3, -61.5, 56.0, -57.0, 53.0, -55.7, 51.5, -60.0, 50.0, -66.0, 49.3,
		-64.5, 48.0, -61.0, 45.5, -66.0, 44.5, -70.0, 43.7, -70.5, 41.8, -74.0, 40.5, -76.0, 38.0, -75.5, 35.5,
		-78.0, 33.8, -81.0, 31.5, -80.2, 27.0, -80.5, 25.2, -81.8, 26.5, -82.8, 28.0, -84.0, 30.0, -87.0, 30.4,
		-89.5, 30.0, -90.0, 29.0, -94.0, 29.6, -97.2, 27.5, -97.5, 24.0, -97.7, 21.5, -96.2, 19.0, -94.5, 18.2,
		-91.5, 18.6, -90.4, 21.0, -87.0, 21.5, -87.6, 18.2, -88.3, 15.8, -84.0, 15.8, -83.2, 14.5, -83.7, 11.0,
		-82.0, 9.0, -79.5, 9.5, -77.4, 8.6, -78.4, 7.6, -80.5, 7.3, -82.0, 8.2, -85.7, 9.9, -87.5, 13.0,
		-91.4, 13.9, -94.0, 16.0, -96.5, 15.7, -101.0, 17.3, -105.5, 20.4, -105.3, 22.5, -109.5, 26.5, -112.5, 29.8,
		-114.7, 31.6, -114.0, 30.0, -110.0, 24.0, -109.5, 23.0, -111.0, 24.5, -112.2, 27.5, -114.2, 29.8, -115.8, 30.5,
		-117.1, 32.5, -118.5, 34.0, -120.6, 34.6, -122.5, 37.5, -124.0, 40.3, -124.5, 43.0, -124.0, 46.3, -124.7, 48.4,
		-123.0, 49.0, -125.5, 50.5, -128.0, 52.0, -130.5, 54.5, -133.0, 57.0, -136.5, 58.3, -140.0, 59.8, -146.0, 60.6,
		-150.0, 59.4, -152.0, 57.6, -157.0, 58.5, -162.0, 58.6, -164.5, 60.5, -165.3, 62.5, -161.0, 64.5, -166.5, 65.2,
	],
	// Greenland
	[
		-73.0, 78.0, -68.0, 80.5, -60.0, 82.0, -45.0, 83.0, -30.0, 83.4, -20.0, 82.0, -18.0, 79.5, -19.5, 76.0,
		-21.5, 72.5, -22.0, 70.2, -26.0, 68.5, -32.0, 68.0, -36.5, 65.8, -40.0, 65.0, -42.0, 62.0, -43.5, 60.0,
		-46.0, 60.8, -48.5, 61.5, -50.5, 64.0, -52.0, 66.0, -53.5, 68.5, -51.0, 70.0, -54.5, 71.5, -55.5, 73.5,
		-58.0, 75.8, -62.0, 76.3, -67.0, 76.5, -71.0, 77.5,
	],
	// Baffin Island
	[
		-62.0, 66.5, -64.0, 64.0, -68.0, 62.5, -71.5, 63.5, -74.0, 65.5, -74.0, 67.5, -77.5, 69.5, -81.0, 70.0,
		-89.0, 73.0, -80.0, 73.7, -75.0, 72.5, -70.0, 71.0, -66.0, 68.8,
	],
	// Canadian Arctic Archipelago (coarse)
	[
		-95.0, 74.0, -82.0, 74.5, -76.0, 76.0, -78.0, 78.0, -70.0, 80.0, -62.0, 82.0, -80.0, 83.0, -92.0, 81.5,
		-100.0, 79.0, -112.0, 78.0, -120.0, 76.5, -125.0, 74.0, -117.0, 72.5, -110.0, 72.8, -105.0, 70.0, -98.0, 70.0,
		-95.0, 72.0,
	],
	// Cuba, Hispaniola
	[
		-85.0, 21.9, -82.0, 23.2, -77.5, 22.2, -74.2, 20.2, -77.5, 19.9, -80.5, 21.8, -84.9, 21.8,
	],
	[
		-74.4, 18.4, -72.8, 19.9, -68.5, 18.8, -68.4, 18.3, -71.0, 17.6, -74.4, 18.3,
	],
	// South America
	[
		-77.5, 8.6, -75.5, 10.5, -72.0, 12.2, -71.5, 10.5, -68.0, 10.6, -64.0, 10.6, -61.0, 10.2, -60.0, 8.5,
		-57.0, 6.0, -53.5, 5.5, -51.0, 4.0, -50.0, 1.5, -48.5, -1.0, -44.5, -2.5, -40.0, -2.8, -37.0, -4.8,
		-35.0, -7.0, -35.0, -9.0, -37.0, -11.5, -39.0, -13.5, -39.0, -17.5, -40.0, -20.5, -41.0, -22.0, -43.0, -23.0,
		-46.5, -24.0, -48.5, -26.5, -48.7, -28.5, -50.5, -31.0, -53.0, -33.8, -55.0, -34.9, -57.5, -34.5, -58.4, -36.3,
		-57.5, -38.0, -62.0, -39.0, -62.3, -41.0, -65.0, -41.2, -63.8, -42.5, -65.5, -45.0, -67.5, -46.5, -65.8, -48.0,
		-68.5, -50.5, -69.0, -52.2, -68.5, -54.5, -71.0, -55.0, -74.0, -52.5, -75.5, -48.5, -74.0, -44.0, -73.5, -40.0,
		-73.3, -37.0, -71.6, -33.0, -71.5, -28.5, -70.4, -23.0, -70.2, -18.4, -72.0, -17.0, -76.0, -14.5, -77.5, -12.0,
		-79.5, -7.5, -81.2, -6.0, -80.3, -3.4, -80.5, -1.0, -80.0, 1.0, -78.8, 2.0, -77.3, 4.0, -77.4, 6.7,
	],
	// Eurasia
	[
		-5.6, 36.0, -9.0, 37.0, -9.5, 39.0, -8.8, 42.0, -9.3, 43.2, -8.0, 43.7, -4.0, 43.4, -1.8, 43.4,
		-1.2, 46.0, -2.2, 47.2, -4.6, 48.0, -1.6, 48.7, 0.0, 49.6, 1.5, 50.2, 2.5, 51.0, 4.0, 51.5,
		4.8, 53.0, 7.0, 53.5, 8.5, 53.8, 8.6, 55.5, 8.2, 57.0, 10.5, 57.7, 10.5, 56.2, 10.9, 54.4,
		12.5, 54.4, 14.2, 53.9, 18.0, 54.8, 19.8, 54.5, 21.0, 56.5, 21.6, 57.5, 24.0, 57.3, 23.5, 59.2,
		28.0, 59.5, 30.0, 60.0, 26.5, 60.4, 22.8, 59.9, 21.3, 60.8, 21.5, 63.0, 25.0, 65.0, 25.4, 65.6,
		22.0, 65.8, 21.0, 64.3, 19.0, 63.2, 17.5, 62.0, 17.3, 60.6, 19.0, 59.8, 18.0, 59.0, 16.5, 57.0,
		16.0, 56.2, 14.4, 55.5, 12.8, 55.9, 12.4, 56.6, 11.0, 58.8, 9.8, 59.0, 8.0, 58.1, 6.0, 58.2,
		5.0, 59.5, 5.0, 61.5, 5.5, 62.5, 8.5, 63.5, 10.5, 64.5, 12.5, 66.0, 14.0, 67.8, 16.0, 68.8,
		19.0, 70.0, 23.0, 70.6, 26.0, 71.0, 29.0, 70.8, 31.0, 70.0, 33.0, 69.3, 36.0, 69.0, 41.0, 67.5,
		41.5, 66.3, 38.0, 66.0, 34.0, 66.5, 32.5, 64.5, 36.0, 64.2, 37.5, 64.8, 40.0, 64.6, 44.0, 66.2,
		44.0, 68.4, 46.0, 68.3, 53.0, 68.8, 55.0, 68.4, 60.0, 68.8, 65.0, 69.3, 68.5, 68.2, 66.8, 70.8,
		69.0, 73.0, 73.0, 72.5, 72.5, 69.0, 75.0, 68.3, 74.0, 72.0, 78.0, 72.4, 80.5, 73.6, 86.5, 74.5,
		87.0, 75.5, 92.0, 76.0, 100.0, 76.5, 104.5, 77.7, 107.0, 77.0, 112.0, 76.0, 114.0, 74.0, 118.0, 73.5,
		124.0, 73.5, 129.0, 72.5, 132.0, 71.5, 139.0, 71.5, 141.0, 72.6, 146.0, 72.3, 150.0, 71.5, 152.0, 70.8,
		157.0, 71.0, 160.0, 70.0, 165.0, 69.5, 170.0, 70.0, 176.0, 69.7, 180.0, 69.0, 180.0, 65.5, 177.0, 64.7,
		178.5, 62.6, 173.0, 61.0, 170.0, 60.0, 165.0, 59.8, 163.0, 58.0, 162.0, 56.2, 156.7, 51.0, 156.0, 53.0,
		155.5, 56.5, 158.0, 58.0, 160.5, 61.3, 158.0, 61.8, 154.5, 59.3, 150.0, 59.5, 143.0, 59.3, 137.5, 54.0,
		141.0, 53.0, 141.3, 48.5, 138.5, 46.5, 136.0, 43.8, 133.0, 42.8, 130.7, 42.4, 129.6, 40.8, 128.0, 39.0,
		129.4, 37.0, 129.3, 35.3, 126.5, 34.4, 126.2, 37.5, 125.0, 39.5, 121.6, 39.0, 121.5, 40.8, 119.0, 39.2,
		117.8, 38.8, 118.8, 37.3, 120.8, 37.8, 122.5, 37.0, 120.0, 35.5, 119.2, 34.5, 121.0, 32.0, 122.0, 30.0,
		121.5, 28.0, 119.5, 25.5, 117.0, 23.6, 114.0, 22.3, 110.5, 21.0, 110.0, 20.3, 108.5, 21.6, 106.5, 20.0,
		105.7, 18.5, 108.8, 15.5, 109.3, 12.0, 107.0, 10.5, 104.8, 8.6, 104.8, 10.4, 103.0, 10.6, 100.5, 12.5,
		100.0, 13.4, 99.2, 10.3, 100.4, 7.3, 101.3, 6.8, 103.4, 4.5, 104.2, 1.4, 103.5, 1.3, 101.3, 2.9,
		100.3, 5.4, 98.3, 8.2, 98.5, 10.5, 98.7, 13.0, 97.7, 16.5, 94.5, 16.0, 94.2, 18.8, 92.3, 21.0,
		91.8, 22.5, 90.5, 22.0, 88.5, 21.6, 87.0, 21.4, 86.5, 20.0, 85.0, 19.3, 82.3, 16.6, 80.3, 15.5,
		80.2, 13.0, 79.8, 10.3, 78.0, 8.3, 77.0, 8.3, 76.0, 10.5, 74.5, 14.0, 73.2, 17.5, 72.8, 21.0,
		72.5, 21.7, 70.0, 20.8, 68.6, 23.0, 67.3, 24.6, 66.6, 25.4, 61.5, 25.2, 57.3, 25.8, 56.4, 27.1,
		54.5, 26.6, 52.0, 27.8, 50.2, 30.0, 48.8, 30.0, 48.0, 29.9, 47.9, 29.0, 49.5, 27.0, 50.1, 26.2,
		51.3, 26.1, 51.6, 24.3, 54.6, 24.3, 56.3, 26.3, 56.4, 24.9, 57.8, 23.6, 59.8, 22.4, 58.5, 20.5,
		57.7, 19.0, 55.5, 17.5, 52.3, 16.3, 48.7, 14.0, 45.0, 12.7, 43.5, 12.7, 42.7, 15.6, 42.7, 16.8,
		41.2, 19.2, 39.1, 21.8, 38.4, 24.0, 36.6, 26.0, 35.0, 28.0, 34.95, 29.5, 34.3, 27.8, 32.6, 29.9,
		32.3, 31.3, 34.3, 31.3, 35.0, 33.0, 35.8, 34.6, 35.9, 35.9, 36.2, 36.6, 34.5, 36.8, 32.5, 36.1,
		30.5, 36.3, 28.2, 36.7, 26.4, 38.2, 26.2, 39.5, 26.6, 40.4, 26.0, 40.8, 23.8, 40.4, 22.6, 40.5,
		23.0, 39.3, 22.8, 37.5, 22.0, 36.5, 21.6, 37.2, 21.1, 38.3, 20.2, 39.7, 19.4, 41.8, 18.5, 42.5,
		16.0, 43.5, 14.5, 45.3, 13.6, 45.8, 12.3, 45.3, 12.5, 44.3, 13.6, 43.5, 16.0, 41.9, 18.5, 40.2,
		17.0, 39.0, 16.6, 38.4, 16.0, 38.0, 15.6, 38.3, 16.2, 39.5, 15.6, 40.1, 14.0, 40.8, 12.4, 41.7,
		11.1, 42.4, 10.2, 43.9, 8.7, 44.4, 7.5, 43.8, 6.2, 43.1, 4.6, 43.4, 3.1, 43.0, 3.2, 41.9,
		1.0, 41.0, 0.0, 39.5, -0.3, 39.4, 0.2, 38.8, -0.7, 37.6, -2.1, 36.7, -4.4, 36.7,
	],
	// Black Sea and Caspian (holes)
	[
		29.0, 41.2, 28.0, 42.0, 27.7, 43.5, 28.6, 44.3, 29.7, 45.3, 30.8, 46.5, 32.0, 46.5, 33.0, 46.0,
		32.5, 45.4, 33.5, 44.5, 35.5, 45.1, 36.5, 45.3, 37.5, 44.7, 39.5, 43.4, 41.6, 41.6, 40.0, 41.0,
		37.0, 41.1, 35.0, 42.0, 33.0, 42.0, 31.0, 41.2,
	],
	[
		47.0, 44.8, 49.0, 46.5, 51.5, 47.0, 53.0, 46.0, 53.0, 45.0, 51.3, 44.5, 52.6, 42.0, 54.0, 40.8,
		53.0, 39.0, 53.9, 37.3, 51.0, 36.7, 49.0, 37.6, 49.5, 40.3, 48.0, 42.0, 47.5, 43.0,
	],
	// Mediterranean islands
	[
		12.4, 38.0, 15.6, 38.3, 15.1, 36.7, 12.4, 37.6,
	],
	[
		8.4, 41.0, 9.8, 41.0, 9.6, 39.0, 8.4, 39.0,
	],
	[
		8.6, 43.0, 9.5, 43.0, 9.4, 41.4, 8.7, 41.6,
	],
	[
		23.5, 35.6, 26.3, 35.3, 26.0, 35.0, 23.5, 35.2,
	],
	[
		32.3, 35.0, 34.6, 35.6, 33.9, 34.6, 32.4, 34.7,
	],
	// Great Britain, Ireland, Iceland
	[
		-5.7, 50.0, -3.0, 50.6, 1.4, 51.2, 1.7, 52.7, 0.3, 53.4, -0.2, 54.2, -1.6, 55.6, -2.1, 57.7,
		-3.3, 58.6, -5.0, 58.6, -6.2, 57.5, -5.6, 56.3, -6.3, 55.4, -4.9, 54.8, -3.4, 54.9, -3.1, 53.9,
		-3.0, 53.3, -4.6, 53.3, -4.2, 52.3, -5.3, 51.8, -3.3, 51.4, -4.5, 51.2,
	],
	[
		-6.0, 52.2, -6.2, 53.9, -5.6, 54.6, -6.2, 55.2, -7.3, 55.3, -8.5, 54.9, -10.0, 54.0, -10.0, 53.0,
		-10.4, 51.8, -9.5, 51.5, -8.0, 51.8,
	],
	[
		-24.0, 65.5, -22.0, 66.4, -18.0, 66.2, -14.5, 66.4, -13.5, 65.0, -15.0, 64.3, -18.7, 63.4, -22.5, 63.8,
		-24.0, 64.8,
	],
	// Svalbard, Novaya Zemlya
	[
		11.0, 78.5, 16.0, 80.0, 22.0, 80.3, 27.0, 80.0, 23.0, 77.3, 17.0, 76.6, 14.0, 77.5,
	],
	[
		52.0, 71.5, 56.0, 74.0, 62.0, 76.5, 69.0, 77.0, 65.0, 75.5, 57.0, 70.6,
	],
	// Africa, Madagascar
	[
		-5.9, 35.8, -2.0, 35.1, 1.0, 36.5, 5.0, 36.8, 9.8, 37.3, 11.0, 37.0, 10.2, 35.5, 11.2, 33.2,
		15.3, 32.3, 18.5, 30.4, 20.0, 31.0, 20.0, 32.2, 22.5, 32.7, 25.0, 31.6, 29.0, 30.9, 32.3, 31.3,
		32.6, 29.9, 33.5, 28.0, 35.5, 24.0, 37.3, 21.0, 38.5, 18.0, 39.5, 15.5, 41.5, 13.5, 43.3, 12.5,
		44.5, 10.4, 47.0, 11.2, 51.2, 11.8, 51.0, 10.5, 49.5, 7.5, 48.0, 4.5, 46.0, 2.0, 43.5, -0.5,
		41.5, -2.0, 39.5, -4.5, 39.0, -6.8, 39.5, -10.0, 40.5, -11.0, 40.6, -15.0, 37.0, -17.5, 35.0, -20.0,
		35.5, -24.0, 32.9, -25.9, 32.5, -28.5, 31.0, -30.0, 28.0, -33.0, 25.6, -34.0, 22.0, -34.2, 20.0, -34.8,
		18.4, -34.0, 18.0, -32.0, 16.5, -28.6, 15.0, -27.0, 14.5, -23.0, 13.4, -20.8, 11.8, -17.5, 12.4, -13.0,
		13.8, -11.0, 13.2, -9.0, 12.3, -6.0, 12.1, -5.0, 9.5, -2.0, 9.3, 1.0, 9.8, 3.8, 8.5, 4.5,
		6.5, 4.3, 5.0, 5.8, 3.0, 6.4, 1.0, 6.0, -2.0, 4.8, -4.0, 5.2, -7.5, 4.4, -9.0, 5.0,
		-11.5, 6.9, -13.2, 8.5, -15.0, 10.9, -16.7, 12.4, -17.5, 14.7, -16.5, 16.5, -16.2, 19.5, -17.0, 21.0,
		-16.0, 23.8, -14.8, 25.5, -13.5, 27.7, -11.0, 28.8, -9.8, 30.0, -9.6, 32.5, -6.8, 34.0,
	],
	[
		49.3, -12.0, 50.5, -15.5, 49.5, -17.5, 47.2, -25.0, 45.0, -25.5, 43.6, -23.5, 43.3, -21.5, 44.4, -19.5,
		44.0, -17.0, 46.3, -15.7, 48.0, -14.0,
	],
	// Sri Lanka, Hainan, Taiwan
	[
		79.8, 9.8, 81.0, 8.5, 81.8, 7.0, 81.2, 6.1, 80.0, 6.0, 79.7, 7.5,
	],
	[
		108.7, 19.2, 110.5, 20.1, 111.0, 19.6, 109.5, 18.2,
	],
	[
		120.1, 23.0, 120.8, 21.9, 121.9, 24.5, 121.5, 25.3, 120.6, 24.5,
	],
	// Japan, Sakhalin
	[
		130.0, 31.3, 131.5, 31.4, 132.0, 33.8, 134.8, 33.7, 136.0, 33.5, 137.2, 34.6, 139.0, 34.8, 140.9, 35.7,
		141.0, 38.3, 142.0, 39.5, 141.4, 41.4, 140.0, 40.6, 139.8, 39.0, 138.5, 37.8, 137.0, 37.0, 136.0, 36.0,
		133.0, 35.5, 131.0, 34.4, 129.6, 33.3,
	],
	[
		140.0, 41.5, 141.2, 41.8, 143.3, 42.0, 145.5, 43.3, 144.0, 44.1, 142.0, 45.4, 141.5, 43.2, 140.3, 43.2,
	],
	[
		142.0, 46.0, 143.5, 46.5, 143.0, 49.5, 144.5, 49.0, 143.0, 54.0, 142.5, 54.2, 142.0, 51.0, 141.8, 48.0,
	],
	// Philippines
	[
		120.0, 18.5, 122.3, 18.5, 122.5, 16.5, 121.5, 15.0, 124.0, 13.0, 123.3, 12.8, 122.0, 13.8, 120.6, 14.1,
		120.0, 16.0,
	],
	[
		122.0, 7.0, 125.5, 9.8, 126.6, 7.3, 125.4, 5.6, 123.8, 6.5,
	],
	// Indonesia, New Guinea
	[
		109.0, 1.8, 111.5, 2.8, 113.0, 3.2, 115.5, 5.2, 117.0, 7.0, 119.2, 5.2, 118.3, 4.3, 117.6, 1.2,
		116.6, -1.0, 116.0, -3.8, 114.5, -3.5, 111.5, -3.0, 110.2, -2.9, 109.6, -1.0,
	],
	[
		95.3, 5.6, 97.5, 5.2, 100.3, 2.3, 103.7, -1.0, 106.0, -3.2, 105.8, -5.8, 104.5, -5.9, 102.0, -4.0,
		100.3, -0.8, 98.7, 1.7, 96.5, 3.7,
	],
	[
		105.2, -6.8, 108.0, -6.3, 111.0, -6.4, 112.6, -6.9, 114.5, -7.8, 111.0, -8.2, 108.0, -7.8, 105.5, -7.0,
	],
	[
		119.5, -5.5, 120.5, -5.6, 120.3, -2.8, 121.5, -1.5, 123.3, -1.0, 121.0, 1.0, 124.9, 1.5, 120.8, 1.2,
		119.7, 0.0, 119.0, -3.5,
	],
	[
		131.0, -1.3, 134.0, -0.9, 135.0, -3.3, 138.0, -1.6, 141.0, -2.6, 145.8, -4.6, 147.5, -6.2, 147.9, -8.0,
		150.3, -10.5, 147.0, -10.1, 144.0, -7.8, 142.0, -9.2, 139.0, -8.0, 138.0, -8.4, 137.8, -5.4, 135.0, -4.3,
		132.8, -4.0, 132.0, -2.8,
	],
	// Australia, Tasmania, New Zealand
	[
		113.5, -22.0, 114.0, -26.0, 115.0, -30.0, 115.0, -33.5, 117.5, -35.0, 120.0, -34.0, 124.0, -33.0, 126.0, -32.2,
		129.0, -31.6, 131.2, -31.5, 134.0, -32.5, 135.6, -34.8, 137.5, -33.3, 138.0, -35.6, 139.5, -36.5, 140.6, -38.0,
		143.5, -38.8, 146.3, -39.0, 147.8, -37.9, 150.0, -37.4, 150.8, -34.0, 152.5, -32.0, 153.6, -28.5, 153.0, -25.5,
		150.8, -22.5, 149.0, -20.5, 146.3, -18.8, 145.3, -15.0, 143.5, -14.0, 142.5, -10.7, 141.6, -12.8, 141.6, -16.3,
		139.5, -17.4, 136.7, -15.8, 135.9, -13.3, 136.8, -12.2, 132.6, -11.5, 131.0, -12.2, 129.5, -14.9, 126.2, -14.0,
		124.4, -16.3, 122.2, -17.7, 121.0, -19.5, 118.8, -20.3, 116.7, -20.6,
	],
	[
		144.7, -40.7, 148.3, -40.9, 148.0, -43.2, 146.0, -43.6, 145.2, -42.2,
	],
	[
		172.7, -34.4, 174.6, -36.0, 175.9, -37.4, 178.5, -37.7, 177.0, -39.3, 176.1, -41.3, 174.8, -41.3, 175.0, -39.8,
		173.8, -39.3, 174.6, -37.5,
	],
	[
		172.7, -40.5, 174.3, -41.7, 173.0, -43.5, 171.2, -44.5, 169.0, -46.6, 166.5, -46.0, 168.3, -44.0, 171.3, -41.8,
	],
	// Antarctica
	[
		-180.0, -84.0, -160.0, -78.5, -150.0, -77.0, -140.0, -75.0, -120.0, -74.0, -100.0, -73.0, -80.0, -73.0, -68.0, -70.0,
		-60.0, -64.0, -57.0, -63.5, -62.0, -66.0, -60.0, -74.0, -45.0, -78.0, -30.0, -77.0, -20.0, -73.0, -10.0, -71.0,
		0.0, -70.0, 20.0, -70.0, 40.0, -69.0, 60.0, -67.5, 80.0, -67.0, 100.0, -66.0, 120.0, -66.5, 140.0, -66.5,
		160.0, -70.0, 170.0, -72.0, 165.0, -78.0, 180.0, -78.0, 180.0, -90.0, -180.0, -90.0,
	],
]
