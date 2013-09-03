package revmgo

import (
	"errors"
	"fmt"
	"github.com/robfig/revel"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"reflect"
)

var (
	Session *mgo.Session // Global mgo Session
	Dial    string       // http://godoc.org/labix.org/v2/mgo#Dial
	Method  string       // clone, copy, new http://godoc.org/labix.org/v2/mgo#Session.New
)

func AppInit() {
	// Read configuration.
	var found bool
	if Dial, found = revel.Config.String("revmgo.dial"); !found {
		// Default to 'localhost'
		Dial = "localhost"
	}
	if Method, found = revel.Config.String("db.spec"); !found {
		Method = "clone"
	} else if err := MethodError(Method); err != nil {
		revel.ERROR.Panic(err)
	}
	// Let's try to connect to Mongo DB right upon start but don't raise an
	// error. Error will be handled if there is actually a request
	if Session == nil {
		Session, _ = mgo.Dial(Dial)
	}

	// register the custom bson.ObjectId binder
	objId := bson.NewObjectId()
	revel.TypeBinders[reflect.TypeOf(objId)] = ObjectIdBinder
}

func ControllerInit() {
	revel.InterceptMethod((*MongoController).Begin, revel.BEFORE)
	revel.InterceptMethod((*MongoController).End, revel.FINALLY)
}

type MongoController struct {
	*revel.Controller
	MongoSession *mgo.Session // named MongoSession to avoid collision with revel.Session
}

// Connect to mgo if we haven't already and return a copy/new/clone of the session
func (c *MongoController) Begin() revel.Result {
	// We may not be connected yet if revel was started before Mongo DB or
	// Mongo DB was restarted
	if Session == nil {
		var err error
		if Session, err = mgo.Dial(Dial); err != nil {
			// Extend the error description to include that this is a Mongo Error
			err = fmt.Errorf("Could not connect to Mongo DB. Error: %s", err)
			return c.RenderError(err)
		}
	}
	switch Method {
	case "clone":
		c.MongoSession = Session.Clone()
	case "copy":
		c.MongoSession = Session.Copy()
	case "new":
		c.MongoSession = Session.New()
	}
	return nil
}

// Close the controller session if we have an active one.
func (c *MongoController) End() revel.Result {
	// This is necessary since End() fill be called no matter what (revel.FINALLY)
	// so it may not be connected.
	if c.MongoSession != nil {
		c.MongoSession.Close()
	}
	return nil
}

func MethodError(m string) error {
	switch m {
	case "clone", "copy", "new":
		return nil
	}
	return errors.New("revmgo: Invalid session instantiation method '%s'")
}

// Custom TypeBinder for bson.ObjectId
// Makes additional Id parameters in actions obsolete
var ObjectIdBinder = revel.Binder{
	// Make a ObjectId from a request containing it in string format.
	Bind: revel.ValueBinder(func(val string, typ reflect.Type) reflect.Value {
		if len(val) == 0 {
			return reflect.Zero(typ)
		}
		if bson.IsObjectIdHex(val) {
			objId := bson.ObjectIdHex(val)
			return reflect.ValueOf(objId)
		} else {
			revel.ERROR.Print("ObjectIdBinder.Bind - invalid ObjectId!")
			return reflect.Zero(typ)
		}
	}),
	// Turns ObjectId back to hexString for reverse routing
	Unbind: func(output map[string]string, name string, val interface{}) {
		var hexStr string
		hexStr = fmt.Sprintf("%s", val.(bson.ObjectId).Hex())
		// not sure if this is too carefull but i wouldn't want invalid ObjectIds in my App
		if bson.IsObjectIdHex(hexStr) {
			output[name] = hexStr
		} else {
			revel.ERROR.Print("ObjectIdBinder.Unbind - invalid ObjectId!")
			output[name] = ""
		}
	},
}
